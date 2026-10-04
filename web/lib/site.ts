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
  /** Display name, e.g. "Acme CLI" */
  name: string;
  /** The binary invoked in examples, e.g. "acme" */
  binary: string;
  /** GitHub "owner/repo" */
  repo: string;
  /** One-line hero heading */
  tagline: string;
  /** Hero sub-paragraph */
  description: string;
  /** Small pill above the heading */
  badge: string;
  /** One-line install command shown in the hero */
  installCommand: string;
  /** Feature cards */
  features: Feature[];
  /** Title above the code block */
  exampleTitle: string;
  /** Shell example rendered in the terminal card */
  example: string;
  /** Optional: tech / query languages this CLI speaks (logo strip) */
  compatible?: string[];
  /** Optional: features section heading (default: "Everything, from one binary") */
  featuresTitle?: string;
  /** Optional: features section subheading */
  featuresSubtitle?: string;
  /** Optional: CTA band body (default mentions installing the binary) */
  ctaBody?: string;
  /** Optional: per-site accent expressed as an OKLCH color */
  accent?: string;
  /** Optional: human-readable accent name */
  accentName?: string;
  /** Optional: sRGB equivalent used by generated images and static assets */
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
