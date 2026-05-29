const base =
  process.env.NEXT_PUBLIC_API_BASE_URL ??
  "https://staging-aegis-futures-utk2.encr.app";

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${base}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
    cache: "no-store",
  });
  if (!res.ok) {
    throw new Error(`${path} ${res.status}`);
  }
  return res.json() as Promise<T>;
}
