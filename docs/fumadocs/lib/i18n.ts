import type { I18nConfig } from 'fumadocs-core/i18n';

export const i18n: I18nConfig = {
  defaultLanguage: 'en',
  languages: ['zh', 'en'],
  // 静态导出无 middleware 做语言协商，必须用 'never' 让所有语言带前缀
  // 否则默认语言的链接路径与实际生成文件路径不匹配，导致 404
  hideLocale: 'never',
};
