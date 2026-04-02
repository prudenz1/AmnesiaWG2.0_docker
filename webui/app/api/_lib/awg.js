const API_BASE_URL = process.env.API_BASE_URL || "http://amneziawg:8080";
const API_TOKEN = process.env.API_TOKEN || "";

export async function awgFetch(path, options = {}) {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${API_TOKEN}`,
      ...(options.headers || {})
    },
    cache: "no-store"
  });

  return res;
}
