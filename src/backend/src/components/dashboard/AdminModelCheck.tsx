import { useState } from 'react';
import { adminMutation } from '../../lib/admin-mutation-request';
import { dashboardQueryClient } from '../../lib/dashboard-query-client';
import type { CapabilityRefreshReport } from '../../lib/model-capabilities/refresh';
import { ADMIN_MODEL_CONFIG_URL } from './use-admin-route-save';

function reportMessage(report: CapabilityRefreshReport) {
  const progress = `Checked ${report.probedModels} models (${report.passed} passed, ${report.failed} failed checks).`;
  return report.remaining > 0 ? `${progress} ${report.remaining} left; check again to continue.` : `${progress} Every model is up to date.`;
}

export function AdminModelCheck() {
  const [checking, setChecking] = useState(false);
  const [message, setMessage] = useState('Model lists show only models that passed a live check for their slot.');

  async function checkModels() {
    setChecking(true);
    setMessage('Checking models…');
    try {
      setMessage(reportMessage(await adminMutation<CapabilityRefreshReport>('/api/admin/model-capabilities', 'POST', {})));
    } catch (error) {
      setMessage(`Could not check models: ${error instanceof Error ? error.message : 'unknown error'}`);
    } finally {
      setChecking(false);
      void dashboardQueryClient().invalidateQueries({ queryKey: [ADMIN_MODEL_CONFIG_URL] });
    }
  }

  function startCheck() {
    void checkModels();
  }

  return (
    <section className="model-route-row">
      <header className="model-route-row__header">
        <strong>Model checks</strong>
        <button type="button" className="button" disabled={checking} onClick={startCheck}>Check models now</button>
      </header>
      <p className="model-route-row__details muted" role="status" aria-live="polite">{message}</p>
    </section>
  );
}
