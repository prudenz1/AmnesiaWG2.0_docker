"use client";

import { useEffect, useState } from "react";

export default function HomePage() {
  const [statusText, setStatusText] = useState("Нет данных");
  const [peers, setPeers] = useState([]);
  const [name, setName] = useState("");
  const [keepalive, setKeepalive] = useState(25);
  const [allowedIps, setAllowedIps] = useState("0.0.0.0/0, ::/0");
  const [message, setMessage] = useState("");

  async function refreshStatus() {
    const res = await fetch("/api/status", { cache: "no-store" });
    const text = await res.text();
    setStatusText(text);
  }

  async function refreshPeers() {
    const res = await fetch("/api/peers", { cache: "no-store" });
    if (!res.ok) {
      setMessage(`Ошибка загрузки клиентов: ${await res.text()}`);
      return;
    }
    const list = await res.json();
    setPeers(Array.isArray(list) ? list : []);
  }

  async function createPeer() {
    if (!name.trim()) {
      setMessage("Введите имя клиента");
      return;
    }
    const res = await fetch("/api/peers", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: name.trim(),
        persistentKeepalive: Number(keepalive) || 25,
        allowedIps: allowedIps.trim() || "0.0.0.0/0, ::/0"
      })
    });
    if (!res.ok) {
      setMessage(`Ошибка создания: ${await res.text()}`);
      return;
    }
    setName("");
    setMessage("Клиент создан");
    await refreshPeers();
  }

  async function removePeer(publicKey) {
    const res = await fetch(`/api/peers/${encodeURIComponent(publicKey)}`, { method: "DELETE" });
    if (!res.ok) {
      setMessage(`Ошибка удаления: ${await res.text()}`);
      return;
    }
    setMessage("Клиент удален");
    await refreshPeers();
  }

  function downloadConfig(name) {
    window.open(`/api/peers/by-name/${encodeURIComponent(name)}/config`, "_blank");
  }

  function openQr(publicKey) {
    window.open(`/api/peers/${encodeURIComponent(publicKey)}/qr`, "_blank");
  }

  useEffect(() => {
    refreshStatus();
    refreshPeers();
  }, []);

  return (
    <main>
      <h1>AmneziaWG Web UI (Next.js)</h1>
      <p className="muted">Управление клиентами через API контейнера amneziawg</p>

      <section className="card">
        <h2>Статус сервера</h2>
        <div className="row">
          <button onClick={refreshStatus}>Обновить статус</button>
          <button onClick={refreshPeers} className="secondary">Обновить клиентов</button>
        </div>
        <pre>{statusText}</pre>
      </section>

      <section className="card">
        <h2>Создать клиента</h2>
        <div className="row">
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Имя клиента" />
          <input
            value={keepalive}
            onChange={(e) => setKeepalive(e.target.value)}
            placeholder="Keepalive"
            type="number"
          />
          <input value={allowedIps} onChange={(e) => setAllowedIps(e.target.value)} placeholder="Allowed IPs" />
          <button onClick={createPeer}>Создать</button>
        </div>
        {message && <p className="muted">{message}</p>}
      </section>

      <section className="card">
        <h2>Клиенты</h2>
        {peers.length === 0 && <p className="muted">Клиентов пока нет</p>}
        {peers.map((p) => (
          <div key={p.publicKey} className="peer">
            <div><b>{p.name}</b></div>
            <div className="muted">IP: {p.address}</div>
            <div className="muted">PublicKey: {p.publicKey}</div>
            <div className="row" style={{ marginTop: 8 }}>
              <button className="secondary" onClick={() => downloadConfig(p.name)}>Скачать .conf</button>
              <button className="secondary" onClick={() => openQr(p.publicKey)}>QR</button>
              <button onClick={() => removePeer(p.publicKey)}>Удалить</button>
            </div>
          </div>
        ))}
      </section>
    </main>
  );
}
