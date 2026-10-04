import type { ReactNode } from 'react';

interface LegalPageProps {
  title: string;
  children: ReactNode;
}

export function LegalPage({ title, children }: LegalPageProps) {
  return (
    <article className="legal-page">
      <header className="legal-page__header">
        <h1>{title}</h1>
        <p className="legal-page__effective">Effective June 14, 2026</p>
      </header>
      <div className="legal-page__content">{children}</div>
    </article>
  );
}
