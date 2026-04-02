import { awgFetch } from "../_lib/awg";

export async function GET() {
  const res = await awgFetch("/api/peers");
  const text = await res.text();
  return new Response(text, {
    status: res.status,
    headers: { "Content-Type": res.headers.get("Content-Type") || "application/json" }
  });
}

export async function POST(request) {
  const body = await request.text();
  const res = await awgFetch("/api/peers", {
    method: "POST",
    body
  });
  const text = await res.text();
  return new Response(text, {
    status: res.status,
    headers: { "Content-Type": res.headers.get("Content-Type") || "application/json" }
  });
}
