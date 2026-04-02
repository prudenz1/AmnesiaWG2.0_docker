import { awgFetch } from "../../_lib/awg";

export async function DELETE(_request, { params }) {
  const res = await awgFetch(`/api/peers/${encodeURIComponent(params.publicKey)}`, {
    method: "DELETE"
  });
  const text = await res.text();
  return new Response(text, {
    status: res.status,
    headers: { "Content-Type": res.headers.get("Content-Type") || "text/plain" }
  });
}
