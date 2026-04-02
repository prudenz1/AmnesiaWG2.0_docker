import { awgFetch } from "../../_lib/awg";

export async function GET(request) {
  const publicKey = request.nextUrl.searchParams.get("publicKey");
  if (!publicKey?.trim()) {
    return new Response("publicKey is required", { status: 400 });
  }
  const res = await awgFetch(
    `/api/peers/config?publicKey=${encodeURIComponent(publicKey)}`,
    { headers: { Accept: "text/plain" } }
  );
  const text = await res.text();
  return new Response(text, {
    status: res.status,
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Content-Disposition": res.headers.get("Content-Disposition") || ""
    }
  });
}
