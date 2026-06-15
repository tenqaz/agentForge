import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  allowedDevOrigins: ["127.0.0.1"],
  async rewrites() {
    const apiBaseUrl =
      process.env.AGENTFORGE_API_BASE_URL ?? "http://127.0.0.1:8080";

    return [
      {
        source: "/api/:path*",
        destination: `${apiBaseUrl}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
