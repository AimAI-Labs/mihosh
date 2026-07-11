import { RootProvider } from 'fumadocs-ui/provider/next';
import { i18n } from '@/lib/i18n';
import '../global.css';

export function generateStaticParams() {
  return i18n.languages.map((lang) => ({ lang }));
}

export default async function Layout(props: { children: React.ReactNode; params: Promise<{ lang: string }> }) {
  const params = await props.params;
  return (
    <html lang={params.lang} suppressHydrationWarning>
      <body className="flex flex-col min-h-screen font-sans antialiased" suppressHydrationWarning>
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
