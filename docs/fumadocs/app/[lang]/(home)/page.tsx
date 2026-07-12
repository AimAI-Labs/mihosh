import { i18n } from '@/lib/i18n';
import HomeContent from './home-content';

export function generateStaticParams() {
  return i18n.languages.map((lang) => ({ lang }));
}

export default async function Page(props: { params: Promise<{ lang: string }> }) {
  const params = await props.params;
  return <HomeContent lang={params.lang} />;
}
