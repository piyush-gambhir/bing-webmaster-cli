import {
  Bot,
  ChartLine,
  KeyRound,
  ListChecks,
  Radar,
  Send,
  type LucideIcon,
} from 'lucide-react';

export interface Feature {
  icon: LucideIcon;
  title: string;
  body: string;
  docsLink?: {
    label: string;
    href: string;
  };
}

export interface SiteConfig {
  name: string;
  binary: string;
  repo: string;
  tagline: string;
  description: string;
  badge: string;
  installCommand: string;
  features: Feature[];
  exampleTitle: string;
  example: string;
  compatible?: string[];
  featuresTitle?: string;
  featuresSubtitle?: string;
  ctaBody?: string;
  accent?: string;
  accentName?: string;
  accentHex?: string;
}

export const site: SiteConfig = {
  name: 'Bing Webmaster CLI',
  binary: 'bwt',
  repo: 'piyush-gambhir/bing-webmaster-cli',
  tagline: 'Bing Webmaster Tools from your terminal',
  description:
    'Bing Webmaster CLI is an independent, unofficial open-source CLI for Bing Webmaster Tools. Read search performance and crawl health, submit URLs, sitemaps, and page content, notify search engines with IndexNow, and manage site settings from a scriptable tool built for coding agents and people alike.',
  badge: 'Open-source · Agent-friendly',
  accent: 'oklch(0.72 0.15 355)',
  accentName: 'rose',
  accentHex: '#ec79a9',
  installCommand:
    'curl -fsSL https://raw.githubusercontent.com/piyush-gambhir/bing-webmaster-cli/main/install.sh | sh',
  features: [
    {
      icon: ChartLine,
      title: 'Search performance',
      body: 'Top queries and pages, clicks and impressions by day, and a summary that states its window. Local filters say so in the output.',
      docsLink: {
        label: 'a summary that states its window',
        href: '/docs/commands/performance',
      },
    },
    {
      icon: Send,
      title: 'Submission & IndexNow',
      body: 'Submit URLs, sitemaps, and full page content to Bing, or notify every IndexNow engine at once from a sitemap and a date.',
      docsLink: {
        label: 'notify every IndexNow engine',
        href: '/docs/commands/submission',
      },
    },
    {
      icon: Radar,
      title: 'Crawl health',
      body: 'Daily crawl stats, crawl issues with the bitmask decoded, crawl rate and boost settings, URL details, and Fetch as Bingbot.',
      docsLink: {
        label: 'Fetch as Bingbot',
        href: '/docs/commands/crawl-urls',
      },
    },
    {
      icon: KeyRound,
      title: 'One key, every site',
      body: 'Paste your Bing Webmaster API key once. It is checked before saving, kept in the OS keychain, and works for all your sites.',
      docsLink: {
        label: 'kept in the OS keychain',
        href: '/docs/authentication',
      },
    },
    {
      icon: Bot,
      title: 'Built for agents',
      body: '-o json with structured errors, --read-only, --dry-run, and bwt api methods, which lists every capability as data.',
      docsLink: {
        label: 'structured errors',
        href: '/docs/agents',
      },
    },
    {
      icon: ListChecks,
      title: 'The whole API',
      body: 'All 59 current Bing Webmaster API methods, each mapped to a command and tested against a pinned documentation snapshot.',
      docsLink: {
        label: 'pinned documentation snapshot',
        href: '/docs/compatibility',
      },
    },
  ],
  exampleTitle: 'An eight-line tour',
  example: `# Log in once; one API key covers every site
bwt auth login
# Read traffic and top queries as JSON
bwt stats summary --compare previous -o json
bwt stats queries --sort clicks --limit 20 -o json
# Check crawl health, then tell Bing what changed
bwt crawl issues -o json
bwt submit urls https://example.com/new-post`,
  compatible: [
    'Search performance',
    'Crawl stats',
    'URL Submission',
    'Content Submission',
    'IndexNow',
    'Sitemaps',
    'Keyword research',
    'Fetch as Bingbot',
  ],
  ctaBody:
    'Install the binary, paste your API key, and start reading your Bing data. No runtime, no dependencies.',
};
