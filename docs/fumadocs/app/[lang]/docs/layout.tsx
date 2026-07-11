import { source } from '@/lib/source';
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import { baseOptions } from '@/lib/layout.shared';

export default async function Layout(props: { children: React.ReactNode; params: Promise<{ lang: string }> }) {
  const params = await props.params;
  return (
    <DocsLayout tree={source.pageTree[params.lang]} {...baseOptions()}>
      {props.children}
    </DocsLayout>
  );
}
