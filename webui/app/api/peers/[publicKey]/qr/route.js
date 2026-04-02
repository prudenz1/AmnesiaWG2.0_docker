import { awgFetch } from "../../../_lib/awg";

export async function GET(_request, { params }) {
  const res = await awgFetch(`/api/peers/${encodeURIComponent(params.publicKey)}/qr`, {
    headers: { Accept: "image/png" }
  });
  const bytes = await res.arrayBuffer();
  return new Response(bytes, {
    status: res.status,
    headers: { "Content-Type": "image/png" }
  });
}
