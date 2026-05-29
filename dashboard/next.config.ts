import type { NextConfig } from "next";

const apiBase =
  process.env.NEXT_PUBLIC_API_BASE_URL ??
  "https://staging-aegis-futures-utk2.encr.app";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  env: {
    NEXT_PUBLIC_API_BASE_URL: apiBase,
  },
};

export default nextConfig;
