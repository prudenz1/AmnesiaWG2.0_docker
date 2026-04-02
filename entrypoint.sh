#!/usr/bin/env bash
set -euo pipefail

CONFIG_DIR="${CONFIG_DIR:-/etc/amnezia/amneziawg}"
WG_CONFIG_FILE="${WG_CONFIG_FILE:-$CONFIG_DIR/wg0.conf}"
STATE_DIR="${STATE_DIR:-/var/lib/amneziawg}"
PEERS_DB="${PEERS_DB:-$STATE_DIR/peers.json}"
WG_INTERFACE="${WG_INTERFACE:-wg0}"

SERVER_URL="${SERVER_URL:-127.0.0.1}"
SERVER_PORT="${SERVER_PORT:-51820}"
SUBNET="${SUBNET:-10.13.13.0/24}"
SERVER_ADDRESS="${SERVER_ADDRESS:-10.13.13.1/24}"
DNS="${DNS:-1.1.1.1}"
API_TOKEN="${API_TOKEN:-change-me}"
API_LISTEN="${API_LISTEN:-:8080}"

AWG_JC="${AWG_JC:-5}"
AWG_JMIN="${AWG_JMIN:-10}"
AWG_JMAX="${AWG_JMAX:-50}"
AWG_S1="${AWG_S1:-58}"
AWG_S2="${AWG_S2:-123}"
AWG_S3="${AWG_S3:-27}"
AWG_S4="${AWG_S4:-11}"
AWG_H1="${AWG_H1:-402635029-1611840891}"
AWG_H2="${AWG_H2:-2002448501-2133445208}"
AWG_H3="${AWG_H3:-2138059656-2141120869}"
AWG_H4="${AWG_H4:-2145854220-2147460034}"
AWG_I1="${AWG_I1:-}"
AWG_I2="${AWG_I2:-}"
AWG_I3="${AWG_I3:-}"
AWG_I4="${AWG_I4:-}"
AWG_I5="${AWG_I5:-}"

mkdir -p "$CONFIG_DIR" "$STATE_DIR" /run/amneziawg
touch "$PEERS_DB"

if [[ ! -s "$PEERS_DB" ]]; then
  echo "[]" > "$PEERS_DB"
fi

if [[ ! -s "$WG_CONFIG_FILE" ]]; then
  SERVER_PRIVATE_KEY="$(awg genkey)"
  SERVER_PUBLIC_KEY="$(printf '%s' "$SERVER_PRIVATE_KEY" | awg pubkey)"

  cat > "$WG_CONFIG_FILE" <<EOF
[Interface]
PrivateKey = $SERVER_PRIVATE_KEY
Address = $SERVER_ADDRESS
ListenPort = $SERVER_PORT
PostUp = iptables -t nat -A POSTROUTING -s ${SUBNET} -o eth0 -j MASQUERADE; iptables -A INPUT -p udp --dport ${SERVER_PORT} -j ACCEPT; iptables -A FORWARD -i ${WG_INTERFACE} -j ACCEPT; iptables -A FORWARD -o ${WG_INTERFACE} -j ACCEPT
PostDown = iptables -t nat -D POSTROUTING -s ${SUBNET} -o eth0 -j MASQUERADE; iptables -D INPUT -p udp --dport ${SERVER_PORT} -j ACCEPT; iptables -D FORWARD -i ${WG_INTERFACE} -j ACCEPT; iptables -D FORWARD -o ${WG_INTERFACE} -j ACCEPT
Jc = ${AWG_JC}
Jmin = ${AWG_JMIN}
Jmax = ${AWG_JMAX}
S1 = ${AWG_S1}
S2 = ${AWG_S2}
S3 = ${AWG_S3}
S4 = ${AWG_S4}
H1 = ${AWG_H1}
H2 = ${AWG_H2}
H3 = ${AWG_H3}
H4 = ${AWG_H4}
I1 = ${AWG_I1}
I2 = ${AWG_I2}
I3 = ${AWG_I3}
I4 = ${AWG_I4}
I5 = ${AWG_I5}
EOF

  if [[ -z "$AWG_I1" ]]; then sed -i '/^I1 = /d' "$WG_CONFIG_FILE"; fi
  if [[ -z "$AWG_I2" ]]; then sed -i '/^I2 = /d' "$WG_CONFIG_FILE"; fi
  if [[ -z "$AWG_I3" ]]; then sed -i '/^I3 = /d' "$WG_CONFIG_FILE"; fi
  if [[ -z "$AWG_I4" ]]; then sed -i '/^I4 = /d' "$WG_CONFIG_FILE"; fi
  if [[ -z "$AWG_I5" ]]; then sed -i '/^I5 = /d' "$WG_CONFIG_FILE"; fi

  chmod 600 "$WG_CONFIG_FILE"
  echo "$SERVER_PUBLIC_KEY" > "$STATE_DIR/server_public.key"
fi

# Remove empty CPS signature keys if they exist in persisted config.
sed -i -E '/^I[1-5] =\s*$/d' "$WG_CONFIG_FILE"

export WG_QUICK_USERSPACE_IMPLEMENTATION="${WG_QUICK_USERSPACE_IMPLEMENTATION:-amneziawg-go}"
export PATH="/usr/local/bin:$PATH"

if ! awg show "$WG_INTERFACE" >/dev/null 2>&1; then
  awg-quick up "$WG_INTERFACE"
fi

cleanup() {
  awg-quick down "$WG_INTERFACE" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

exec awg-api \
  --listen "$API_LISTEN" \
  --token "$API_TOKEN" \
  --wg-interface "$WG_INTERFACE" \
  --wg-config "$WG_CONFIG_FILE" \
  --peers-db "$PEERS_DB" \
  --server-url "$SERVER_URL" \
  --server-port "$SERVER_PORT" \
  --dns "$DNS" \
  --subnet "$SUBNET"
