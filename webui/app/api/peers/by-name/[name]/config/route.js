import { awgFetch } from "../../../../_lib/awg";

export async function GET(_request, { params }) {
  const targetName = (params.name || "").trim();
  if (!targetName) {
    return new Response("name is required", { status: 400 });
  }

  const peersRes = await awgFetch("/api/peers");
  if (!peersRes.ok) {
    return new Response(await peersRes.text(), { status: peersRes.status });
  }

  const peers = await peersRes.json();
  const peer = Array.isArray(peers)
    ? peers.find((p) => typeof p?.name === "string" && p.name.trim() === targetName)
    : null;

  if (!peer?.publicKey) {
    return new Response("peer not found", { status: 404 });
  }

  const cfgRes = await awgFetch(
    `/api/peers/config?publicKey=${encodeURIComponent(peer.publicKey)}`,
    { headers: { Accept: "text/plain" } }
  );
  const text = await cfgRes.text();
  return new Response(text, {
    status: cfgRes.status,
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Content-Disposition": cfgRes.headers.get("Content-Disposition") || ""
    }
  });
}
