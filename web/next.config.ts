import type { NextConfig } from "next";
const config: NextConfig = {
  output: "standalone",
  // Allow the bounded learning stream to finish through the API rewrite.
  experimental: { proxyTimeout: 120000 },
  async redirects() {
    return ["science-ladder.fly.dev", "www.scienceladder.org"].flatMap(
      (host) => [
        // Start OAuth on the canonical host so its state cookie follows the callback.
        {
          source: "/v1/auth/github",
          has: [{ type: "host" as const, value: host }],
          destination: "https://scienceladder.org/v1/auth/github",
          permanent: false,
        },
        // Keep legacy API clients, verification keys, and loaded assets reachable.
        {
          source: "/:path((?!v1(?:/|$)|\\.well-known(?:/|$)|_next(?:/|$)).*)",
          has: [{ type: "host" as const, value: host }],
          destination: "https://scienceladder.org/:path",
          permanent: true,
        },
      ],
    );
  },
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
