/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Emit a self-contained server bundle so the production image ships only the
  // runtime, not the full node_modules tree.
  output: 'standalone',
  images: {
    domains: ['assets.coingecko.com', 'coin-images.coingecko.com'],
  },
  webpack: (config) => {
    config.resolve.fallback = { fs: false, net: false, tls: false };
    return config;
  },
};

module.exports = nextConfig;
