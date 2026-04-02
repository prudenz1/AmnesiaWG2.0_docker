import { awgFetch } from "../_lib/awg";

export async function GET() {
  const res = await awgFetch("/api/status");
  const text = await res.text();
  return new Response(text, {
    status: res.status,
    headers: { "Content-Type": res.headers.get("Content-Type") || "application/json" }
  });
}
