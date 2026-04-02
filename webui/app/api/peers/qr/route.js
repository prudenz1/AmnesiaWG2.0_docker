import { awgFetch } from "../../_lib/awg";

export async function GET(request) {
  const publicKey = request.nextUrl.searchParams.get("publicKey");
  if (!publicKey?.trim()) {
    return new Response("publicKey is required", { status: 400 });
  }
  const res = await awgFetch(`/api/peers/qr?publicKey=${encodeURIComponent(publicKey)}`, {
    headers: { Accept: "image/png" }
  });
  const buf = await res.arrayBuffer();
  return new Response(buf, {
    status: res.status,
    headers: { "Content-Type": res.headers.get("Content-Type") || "image/png" }
  });
}
