import { RootProvider } from 'fumadocs-ui/provider/next';
import '../global.css';

export default async function Layout(props: { children: React.ReactNode; params: Promise<{ lang: string }> }) {
  const params = await props.params;
  return (
    <html lang={params.lang} suppressHydrationWarning>
      <body className="flex flex-col min-h-screen font-sans antialiased">
        <RootProvider
          i18n={{
            locale: params.lang,
            locales: [
              { locale: 'zh', name: '简体中文' },
              { locale: 'en', name: 'English' },
            ],
          }}
        >
          {props.children}
        </RootProvider>
      </body>
    </html>
  );
}
