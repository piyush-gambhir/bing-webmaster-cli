import type { Metadata } from 'next';
import Link from 'next/link';
import { LegalPage } from '@/components/legal-page';
import { createPageMetadata } from '@/lib/metadata';

export const metadata: Metadata = createPageMetadata({
  title: 'Privacy Policy',
  description:
    'Privacy policy for Bing Webmaster CLI, an independent, unofficial open-source CLI for Bing Webmaster Tools.',
  path: '/privacy',
});

export default function PrivacyPage() {
  return (
    <LegalPage title="Privacy Policy">
      <p className="legal-page__lede">
        Bing Webmaster CLI (the <code>bwt</code> command-line tool) is an open-source
        program that runs entirely on your own computer. It does <strong>not</strong>{' '}
        collect, transmit, or store your personal data on any server operated by the
        maintainer.
      </p>

      <h2>1. No data collection</h2>
      <p>
        The CLI runs locally on your machine. The maintainer operates{' '}
        <strong>no backend servers</strong> and receives <strong>no data</strong> from
        your use of the tool. There is no analytics, no telemetry, no tracking, and
        no advertising.
      </p>

      <h2>2. Credentials &amp; local storage</h2>
      <p>
        Your Bing Webmaster API key is stored <strong>only on your device</strong>, in
        your operating system&apos;s keychain (macOS Keychain, Windows Credential
        Manager, or the Secret Service on Linux). If you choose{' '}
        <code>--insecure-storage</code>, it is kept in a local file with owner-only{' '}
        <code>0600</code> permissions instead. Profile names, default sites, and
        IndexNow keys (which are public by design) live in{' '}
        <code>~/.config/bing-webmaster-cli/config.yaml</code>. Nothing is sent to the
        maintainer or any third party.
      </p>

      <h2>3. Network connections</h2>
      <p>The CLI makes outbound network requests only when you run a command, and only to:</p>
      <ul>
        <li>
          <strong>The Bing Webmaster API</strong>, to perform the actions you request:{' '}
          <code>ssl.bing.com</code> with your API key, or <code>www.bing.com</code> when you
          supply a bearer token.
        </li>
        <li>
          <strong>An IndexNow endpoint</strong> when you submit URLs with IndexNow:{' '}
          <code>api.indexnow.org</code> by default, <code>www.bing.com</code>, or an HTTPS
          endpoint you name with <code>--endpoint</code>.
        </li>
        <li>
          <strong>Your own site</strong>, to read a sitemap you name or check that your
          IndexNow key file is published.
        </li>
        <li>
          <strong>GitHub</strong>, only when you run <code>bwt update</code>, to find and
          download the latest release. This request contains no personal data.
        </li>
      </ul>
      <p>
        There are no background requests. The maintainer is not a party to, and cannot
        observe, these connections.
      </p>

      <h2>4. Data you access through the tool</h2>
      <p>
        The CLI reads and changes data in your Bing Webmaster Tools account using the
        access your API key grants, solely to execute the commands you run. That data
        is shown in your terminal (or written to files you specify) and is{' '}
        <strong>not</strong> retained, copied, or transmitted anywhere by the
        maintainer.
      </p>

      <h2>5. Third parties</h2>
      <p>
        The maintainer does not sell, rent, or share any data. The tool integrates no
        third-party analytics or tracking SDKs. Your use of Bing Webmaster Tools is
        governed by Microsoft&apos;s privacy statement, and IndexNow submissions by the
        policies of the participating search engines.
      </p>

      <h2>6. Children</h2>
      <p>This tool is a developer utility and is not directed at children under 13.</p>

      <h2>7. Changes to this policy</h2>
      <p>Any changes will be posted on this page with an updated effective date.</p>

      <h2>8. Contact</h2>
      <p>
        Questions about this policy? Contact <strong>Piyush Gambhir</strong> at{' '}
        <a href="mailto:developer.piyushgambhir@gmail.com">
          developer.piyushgambhir@gmail.com
        </a>
        , or see the <Link href="/contact">contact page</Link>.
      </p>
    </LegalPage>
  );
}
