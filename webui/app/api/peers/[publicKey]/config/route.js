import { awgFetch } from "../../../_lib/awg";

export async function GET(_request, { params }) {
  const res = await awgFetch(`/api/peers/${encodeURIComponent(params.publicKey)}/config`, {
    headers: { Accept: "text/plain" }
  });
  const text = await res.text();
  return new Response(text, {
    status: res.status,
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Content-Disposition": res.headers.get("Content-Disposition") || ""
    }
  });
}
