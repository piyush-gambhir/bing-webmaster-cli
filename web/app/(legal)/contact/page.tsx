import type { Metadata } from 'next';
import Link from 'next/link';
import { LegalPage } from '@/components/legal-page';
import { createPageMetadata } from '@/lib/metadata';

export const metadata: Metadata = createPageMetadata({
  title: 'Contact',
  description:
    'Contact information for Bing Webmaster CLI, an independent, unofficial open-source CLI for Bing Webmaster Tools.',
  path: '/contact',
});

export default function ContactPage() {
  return (
    <LegalPage title="Contact">
      <p className="legal-page__lede">
        Bing Webmaster CLI is a free, open-source project maintained by{' '}
        <strong>Piyush Gambhir</strong>. Support is best-effort; here are the best
        ways to get in touch.
      </p>

      <div className="legal-contact-grid">
        <section>
          <p className="legal-contact-grid__label">Email</p>
          <p className="legal-contact-grid__value">
            <a href="mailto:developer.piyushgambhir@gmail.com">
              developer.piyushgambhir@gmail.com
            </a>
          </p>
          <p>General questions, privacy, and security reports.</p>
        </section>
        <section>
          <p className="legal-contact-grid__label">Bugs &amp; features</p>
          <p className="legal-contact-grid__value">
            <a
              href="https://github.com/piyush-gambhir/bing-webmaster-cli/issues"
              target="_blank"
              rel="noreferrer"
            >
              GitHub Issues ↗
            </a>
          </p>
          <p>The fastest way to report a bug or request a feature.</p>
        </section>
        <section>
          <p className="legal-contact-grid__label">Source</p>
          <p className="legal-contact-grid__value">
            <a
              href="https://github.com/piyush-gambhir/bing-webmaster-cli"
              target="_blank"
              rel="noreferrer"
            >
              piyush-gambhir/bing-webmaster-cli ↗
            </a>
          </p>
          <p>Read the code, open a pull request, or fork it.</p>
        </section>
      </div>

      <h2>Security issues</h2>
      <p>
        If you believe you&apos;ve found a security vulnerability, please email{' '}
        <a href="mailto:developer.piyushgambhir@gmail.com">
          developer.piyushgambhir@gmail.com
        </a>{' '}
        with the details rather than opening a public issue, or use GitHub&apos;s private vulnerability
        reporting on the repository. Bing Webmaster CLI keeps your API key only on your own device and
        operates no servers, but responsible
        disclosure is always appreciated.
      </p>

      <h2>Response time</h2>
      <p>
        This is an independent side project, not a commercial product. The maintainer
        aims to respond when possible, but no response time or level of support is
        guaranteed. See the <Link href="/terms">Terms of Service</Link> for the full
        no-warranty terms.
      </p>

      <h2>Not affiliated with Microsoft</h2>
      <p>
        Bing Webmaster CLI is an independent, unofficial tool and is not affiliated
        with, endorsed by, or sponsored by Microsoft. For issues with Bing Webmaster
        Tools itself, use Microsoft&apos;s own support channels.
      </p>
    </LegalPage>
  );
}
