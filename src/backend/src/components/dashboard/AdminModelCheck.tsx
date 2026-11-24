import { useState } from 'react';
import { adminMutation } from '../../lib/admin-mutation-request';
import { dashboardQueryClient } from '../../lib/dashboard-query-client';
import type { CapabilityRefreshReport } from '../../lib/model-capabilities/refresh';
import { Button } from '../ui/button';
import { ADMIN_MODEL_CONFIG_URL } from './use-admin-route-save';

function reportMessage(report: CapabilityRefreshReport) {
  const checked = `Checked ${report.probedModels} models`;
  return report.remaining > 0 ? `${checked}, ${report.remaining} left` : `${checked}, all up to date`;
}

export function AdminModelCheck() {
  const [checking, setChecking] = useState(false);
  const [message, setMessage] = useState('');

  async function checkModels() {
    setChecking(true);
    setMessage('Checking…');
    try {
      setMessage(reportMessage(await adminMutation<CapabilityRefreshReport>('/api/admin/model-capabilities', 'POST', {})));
    } catch (error) {
      setMessage(`Check failed: ${error instanceof Error ? error.message : 'unknown error'}`);
    } finally {
      setChecking(false);
      void dashboardQueryClient().invalidateQueries({ queryKey: [ADMIN_MODEL_CONFIG_URL] });
    }
  }

  function startCheck() {
    void checkModels();
  }

  return (
    <div className="flex items-center gap-3">
      <span className="muted text-xs" role="status" aria-live="polite">{message}</span>
      <Button variant="outline" size="sm" disabled={checking} onClick={startCheck}>Check models</Button>
    </div>
  );
}
