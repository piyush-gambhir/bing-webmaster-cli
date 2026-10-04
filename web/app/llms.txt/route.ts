import { source } from '@/lib/source';
import { llms } from 'fumadocs-core/source';
import { site } from '@/lib/site';
import { siteUrl } from '@/lib/shared';
import { getOtherSuiteProjects } from '@/lib/suite';

export const revalidate = false;

export async function GET() {
  // index() returns a Promise since fumadocs-core 16.15.17.
  const index = (await llms(source).index()).replace(/\]\((\/[^)]+)\)/g, (_match, path: string) => `](${siteUrl}${path})`);
  const relatedSites = getOtherSuiteProjects(site.repo)
    .map(({ name, href }) => `- ${name}: ${href}`)
    .join('\n');
  const intro =
    'Bing Webmaster CLI (binary: bwt) is an independent, unofficial command-line interface for Bing Webmaster Tools, built mainly for coding agents. It covers all 59 non-obsolete methods of the Bing Webmaster JSON API plus IndexNow: search performance, crawl health, URL information, URL, content, and sitemap submission, keyword research, and site settings. Agents should run commands with -o json --no-input, use --read-only unless asked to change something, preview writes with --dry-run, and discover capabilities with bwt api methods -o json.';
  return new Response(
    `${intro}\n\n${index}\n\n## Related CLI sites\n\n${relatedSites}\n`,
  );
}
