export const appName = 'Bing Webmaster CLI';
export const siteUrl = 'https://projects.piyushgambhir.com/bing-webmaster-cli';
export const docsRoute = '/docs';
export const docsImageRoute = '/og/docs';
export const docsContentRoute = '/llms.mdx/docs';

const generatedSources: Record<string, string> = {
  'reference/commands.mdx': 'docs/commands.md',
  'reference/api-coverage.mdx': 'docs/api-coverage.md',
};

export function sourcePath(pagePath: string): string {
  return generatedSources[pagePath] ?? `web/content/docs/${pagePath}`;
}

export const gitConfig = {
  user: 'piyush-gambhir',
  repo: 'bing-webmaster-cli',
  branch: 'main',
};
