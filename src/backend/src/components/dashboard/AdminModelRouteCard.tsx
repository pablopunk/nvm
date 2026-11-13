import type { ReactNode } from 'react';

export type AdminModelTier = 'pro' | 'free' | 'dictation' | 'decisions';
export type RouteSaveStatus = { kind: '' | 'ok' | 'err'; message: string };

export function AdminModelRouteCard({ title, tier, status, details, children }: { title: string; tier: AdminModelTier; status: RouteSaveStatus; details: ReactNode; children: ReactNode }) {
  return (
    <section className="model-route-row" data-tier={tier}>
      <header className="model-route-row__header">
        <strong>{title}</strong>
        <span className={`status${status.kind ? ` status--${status.kind}` : ''}`} role="status" aria-live="polite">{status.message}</span>
      </header>
      <div className="model-route-row__controls">{children}</div>
      <p className="model-route-row__details muted">{details}</p>
    </section>
  );
}
