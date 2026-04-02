package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

type peer struct {
	Name          string    `json:"name"`
	PublicKey     string    `json:"publicKey"`
	PrivateKey    string    `json:"privateKey"`
	PresharedKey  string    `json:"presharedKey"`
	Address       string    `json:"address"`
	CreatedAt     time.Time `json:"createdAt"`
	PersistentKA  int       `json:"persistentKeepalive"`
	AllowedIPs    string    `json:"allowedIps"`
}

type server struct {
	token       string
	wgInterface string
	wgConfig    string
	peersDB     string
	serverURL   string
	serverPort  string
	dns         string
	subnet      *net.IPNet
	mu          sync.Mutex
}

func main() {
	listen := flag.String("listen", ":8080", "listen address")
	token := flag.String("token", "change-me", "bearer token")
	wgInterface := flag.String("wg-interface", "wg0", "wireguard interface")
	wgConfig := flag.String("wg-config", "/etc/amneziawg/wg0.conf", "wireguard config path")
	peersDB := flag.String("peers-db", "/var/lib/amneziawg/peers.json", "peers db path")
	serverURL := flag.String("server-url", "127.0.0.1", "public server address")
	serverPort := flag.String("server-port", "51820", "public udp port")
	dns := flag.String("dns", "1.1.1.1", "client dns")
	subnetStr := flag.String("subnet", "10.13.13.0/24", "vpn subnet")
	flag.Parse()

	_, ipNet, err := net.ParseCIDR(*subnetStr)
	if err != nil {
		log.Fatalf("invalid subnet: %v", err)
	}

	s := &server{
		token:       *token,
		wgInterface: *wgInterface,
		wgConfig:    *wgConfig,
		peersDB:     *peersDB,
		serverURL:   *serverURL,
		serverPort:  *serverPort,
		dns:         *dns,
		subnet:      ipNet,
	}

	if err := s.ensurePeersDB(); err != nil {
		log.Fatalf("init peers db: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.auth(s.handleStatus))
	mux.HandleFunc("/api/peers", s.auth(s.handlePeers))
	mux.HandleFunc("/api/reload", s.auth(s.handleReload))
	mux.HandleFunc("/api/peers/", s.auth(s.handlePeerRoutes))

	log.Printf("awg-api listening on %s", *listen)
	log.Fatal(http.ListenAndServe(*listen, s.logMiddleware(mux)))
}

func (s *server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func (s *server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authz := strings.TrimSpace(r.Header.Get("Authorization"))
		if authz == "" || !strings.HasPrefix(authz, "Bearer ") || strings.TrimPrefix(authz, "Bearer ") != s.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	out, err := runCmd("awg", "show", s.wgInterface, "dump")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"interface": s.wgInterface,
		"serverUrl": s.serverURL,
		"serverPort": s.serverPort,
		"dump": strings.TrimSpace(out),
		"time": time.Now().UTC(),
	})
}

func (s *server) handlePeers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		peers, err := s.readPeers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, peers)
	case http.MethodPost:
		s.createPeer(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) handlePeerRoutes(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/peers/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	pub := parts[0]

	if len(parts) == 1 && r.Method == http.MethodDelete {
		s.deletePeer(w, pub)
		return
	}

	if len(parts) == 2 && parts[1] == "config" && r.Method == http.MethodGet {
		s.getPeerConfig(w, pub)
		return
	}

	if len(parts) == 2 && parts[1] == "qr" && r.Method == http.MethodGet {
		s.getPeerQR(w, pub)
		return
	}

	http.NotFound(w, r)
}

func (s *server) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, err := runCmd("awg", "syncconf", s.wgInterface, s.wgConfig); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reloaded"})
}

type createPeerReq struct {
	Name                 string `json:"name"`
	PersistentKeepalive  int    `json:"persistentKeepalive"`
	AllowedIPs           string `json:"allowedIps"`
}

func (s *server) createPeer(w http.ResponseWriter, r *http.Request) {
	var req createPeerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.PersistentKeepalive == 0 {
		req.PersistentKeepalive = 25
	}
	if req.AllowedIPs == "" {
		req.AllowedIPs = "0.0.0.0/0, ::/0"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	peers, err := s.readPeers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	nextIP, err := s.nextIP(peers)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	privateKey, err := runCmd("awg", "genkey")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	privateKey = strings.TrimSpace(privateKey)

	publicKey, err := runCmdInput(privateKey, "awg", "pubkey")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	publicKey = strings.TrimSpace(publicKey)

	presharedKey, err := runCmd("awg", "genpsk")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	presharedKey = strings.TrimSpace(presharedKey)

	presharedPath := filepath.Join(os.TempDir(), "psk-"+safeName(publicKey)+".key")
	if err := os.WriteFile(presharedPath, []byte(presharedKey), 0600); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(presharedPath)

	if _, err := runCmd(
		"awg", "set", s.wgInterface,
		"peer", publicKey,
		"preshared-key", presharedPath,
		"allowed-ips", nextIP+"/32",
		"persistent-keepalive", strconv.Itoa(req.PersistentKeepalive),
	); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	np := peer{
		Name:         req.Name,
		PublicKey:    publicKey,
		PrivateKey:   privateKey,
		PresharedKey: presharedKey,
		Address:      nextIP,
		CreatedAt:    time.Now().UTC(),
		PersistentKA: req.PersistentKeepalive,
		AllowedIPs:   req.AllowedIPs,
	}

	peers = append(peers, np)
	if err := s.writePeers(peers); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.appendPeerToConfig(np); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, np)
}

func (s *server) deletePeer(w http.ResponseWriter, pub string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	peers, err := s.readPeers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idx := -1
	for i := range peers {
		if peers[i].PublicKey == pub {
			idx = i
			break
		}
	}
	if idx < 0 {
		http.Error(w, "peer not found", http.StatusNotFound)
		return
	}

	if _, err := runCmd("awg", "set", s.wgInterface, "peer", pub, "remove"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	peers = append(peers[:idx], peers[idx+1:]...)
	if err := s.writePeers(peers); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.rewriteConfigWithPeers(peers); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *server) getPeerConfig(w http.ResponseWriter, pub string) {
	p, err := s.findPeer(pub)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	serverPub, err := s.serverPublicKey()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	awgExtra, err := s.readAWGInterfaceParams()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var b strings.Builder
	b.WriteString("[Interface]\n")
	b.WriteString("PrivateKey = " + p.PrivateKey + "\n")
	b.WriteString("Address = " + p.Address + "/32\n")
	b.WriteString("DNS = " + s.dns + "\n")
	if awgExtra != "" {
		b.WriteString(awgExtra)
		if !strings.HasSuffix(awgExtra, "\n") {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n[Peer]\n")
	b.WriteString("PublicKey = " + serverPub + "\n")
	b.WriteString("PresharedKey = " + p.PresharedKey + "\n")
	b.WriteString("Endpoint = " + s.serverURL + ":" + s.serverPort + "\n")
	b.WriteString("AllowedIPs = " + p.AllowedIPs + "\n")
	b.WriteString("PersistentKeepalive = " + strconv.Itoa(p.PersistentKA) + "\n")

	cfg := b.String()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeName(p.Name)+`.conf"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(cfg))
}

func (s *server) getPeerQR(w http.ResponseWriter, pub string) {
	cfgBuf := bytes.NewBuffer(nil)
	rec := responseRecorder{header: http.Header{}}
	s.getPeerConfig(&rec, pub)
	if rec.status != http.StatusOK {
		http.Error(w, rec.body.String(), rec.status)
		return
	}
	cfgBuf.Write(rec.body.Bytes())

	png, err := qrcode.Encode(cfgBuf.String(), qrcode.Medium, 512)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (s *server) ensurePeersDB() error {
	dir := filepath.Dir(s.peersDB)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if _, err := os.Stat(s.peersDB); err == nil {
		return nil
	}
	return os.WriteFile(s.peersDB, []byte("[]"), 0644)
}

func (s *server) readPeers() ([]peer, error) {
	raw, err := os.ReadFile(s.peersDB)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return []peer{}, nil
	}
	var peers []peer
	if err := json.Unmarshal(raw, &peers); err != nil {
		return nil, err
	}
	return peers, nil
}

func (s *server) writePeers(peers []peer) error {
	raw, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.peersDB, raw, 0644)
}

func (s *server) findPeer(pub string) (*peer, error) {
	peers, err := s.readPeers()
	if err != nil {
		return nil, err
	}
	for i := range peers {
		if peers[i].PublicKey == pub {
			return &peers[i], nil
		}
	}
	return nil, os.ErrNotExist
}

func (s *server) serverPublicKey() (string, error) {
	out, err := runCmd("awg", "show", s.wgInterface, "public-key")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *server) readAWGInterfaceParams() (string, error) {
	raw, err := os.ReadFile(s.wgConfig)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(raw), "\n")
	keys := map[string]bool{
		"Jc": true, "Jmin": true, "Jmax": true,
		"S1": true, "S2": true, "S3": true, "S4": true,
		"H1": true, "H2": true, "H3": true, "H4": true,
		"I1": true, "I2": true, "I3": true, "I4": true, "I5": true,
	}
	var out []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "[") {
			continue
		}
		parts := strings.SplitN(l, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		if keys[k] {
			out = append(out, fmt.Sprintf("%s = %s", k, strings.TrimSpace(parts[1])))
		}
	}
	return strings.Join(out, "\n"), nil
}

func (s *server) appendPeerToConfig(p peer) error {
	f, err := os.OpenFile(s.wgConfig, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	block := fmt.Sprintf(`
[Peer]
# Name = %s
PublicKey = %s
PresharedKey = %s
AllowedIPs = %s/32
PersistentKeepalive = %d
`, p.Name, p.PublicKey, p.PresharedKey, p.Address, p.PersistentKA)

	_, err = f.WriteString(block)
	return err
}

func (s *server) rewriteConfigWithPeers(peers []peer) error {
	raw, err := os.ReadFile(s.wgConfig)
	if err != nil {
		return err
	}
	text := string(raw)
	idx := strings.Index(text, "\n[Peer]\n")
	base := text
	if idx >= 0 {
		base = text[:idx]
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(base, "\n"))
	b.WriteString("\n")
	for _, p := range peers {
		b.WriteString(fmt.Sprintf(`
[Peer]
# Name = %s
PublicKey = %s
PresharedKey = %s
AllowedIPs = %s/32
PersistentKeepalive = %d
`, p.Name, p.PublicKey, p.PresharedKey, p.Address, p.PersistentKA))
	}
	return os.WriteFile(s.wgConfig, []byte(b.String()), 0600)
}

func (s *server) nextIP(peers []peer) (string, error) {
	used := map[string]bool{}
	for _, p := range peers {
		used[p.Address] = true
	}
	base := s.subnet.IP.To4()
	if base == nil {
		return "", fmt.Errorf("only IPv4 subnet is supported")
	}
	for i := 2; i < 255; i++ {
		ip := net.IPv4(base[0], base[1], base[2], byte(i)).String()
		if !used[ip] {
			return ip, nil
		}
	}
	return "", fmt.Errorf("no free IPs in subnet")
}

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s %v failed: %s", name, args, msg)
	}
	return out.String(), nil
}

func runCmdInput(input string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(input)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s %v failed: %s", name, args, msg)
	}
	return out.String(), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func safeName(name string) string {
	out := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)
	if out == "" {
		return "client"
	}
	return out
}

type responseRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}
