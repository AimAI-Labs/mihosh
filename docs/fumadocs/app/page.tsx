// 静态导出时无 proxy/middleware 做语言协商，根路由需静态重定向到默认语言
// dev 模式下 proxy 会 rewrite / → /en，此页面不会被命中
export default function RootPage() {
  return (
    <>
      <meta httpEquiv="refresh" content="0;url=en" />
      <p>Redirecting to <a href="en">English</a>...</p>
    </>
  );
}
