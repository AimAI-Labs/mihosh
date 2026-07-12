import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();

/** @type {import('next').NextConfig} */
const config = {
  reactStrictMode: true,
  allowedDevOrigins: ['*.*.*.*'],
  // 静态导出与 middleware/proxy 不兼容，仅在生产构建时通过 STATIC_EXPORT=true 启用
  output: process.env.STATIC_EXPORT === 'true' ? 'export' : undefined,
  images: {
    unoptimized: true,
  }
};

export default withMDX(config);
