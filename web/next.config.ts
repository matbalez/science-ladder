import type { NextConfig } from "next";
const config: NextConfig = {
  output: "standalone",
  async rewrites() {
    const api = (process.env.API_URL || "http://127.0.0.1:8080").replace(
      /\/$/,
      "",
    );
    return [
      { source: "/v1/:path*", destination: `${api}/v1/:path*` },
      {
        source: "/.well-known/science-ladder-keys.json",
        destination: `${api}/.well-known/science-ladder-keys.json`,
      },
    ];
  },
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "X-Frame-Options", value: "DENY" },
          {
            key: "Permissions-Policy",
            value: "camera=(), microphone=(), geolocation=()",
          },
        ],
      },
    ];
  },
};
export default config;
