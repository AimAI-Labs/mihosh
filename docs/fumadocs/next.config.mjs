import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();

/** @type {import('next').NextConfig} */
const config = {
  reactStrictMode: true,
  allowedDevOrigins: ['*.*.*.*'],
  // 静态导出与 middleware/proxy 不兼容，CI 中由 configure-pages 设置 GITHUB_PAGES=true 启用
  output: process.env.GITHUB_PAGES === 'true' ? 'export' : undefined,
  // GitHub Pages 项目页需 basePath=/mihosh，configure-pages 可能无法解析 withMDX 包装的配置
  basePath: process.env.GITHUB_PAGES === 'true' ? '/mihosh' : '',
  images: {
    unoptimized: true,
  }
};

export default withMDX(config);
