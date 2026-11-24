import { useState } from 'react';
import { adminMutation } from '../../lib/admin-mutation-request';
import { dashboardQueryClient } from '../../lib/dashboard-query-client';
import type { CapabilityRefreshReport } from '../../lib/model-capabilities/refresh';
import { Button } from '../ui/button';
import { ADMIN_MODEL_CONFIG_URL } from './use-admin-route-save';

const MAX_CHECK_ROUNDS = 40;

function progressMessage(checked: number, remaining: number) {
  return remaining > 0 ? `Checked ${checked} models, ${remaining} left…` : `Checked ${checked} models, all up to date`;
}

function runCheckRound() {
  return adminMutation<CapabilityRefreshReport>('/api/admin/model-capabilities', 'POST', {});
}

function refreshModelLists() {
  return dashboardQueryClient().invalidateQueries({ queryKey: [ADMIN_MODEL_CONFIG_URL] });
}

export function AdminModelCheck() {
  const [checking, setChecking] = useState(false);
  const [message, setMessage] = useState('');

  async function checkModels() {
    setChecking(true);
    setMessage('Checking…');
    let checked = 0;
    try {
      for (let round = 0; round < MAX_CHECK_ROUNDS; round += 1) {
        const report = await runCheckRound();
        checked += report.probedModels;
        setMessage(progressMessage(checked, report.remaining));
        await refreshModelLists();
        if (report.remaining === 0 || report.probedModels === 0) break;
      }
    } catch (error) {
      setMessage(`Check stopped after ${checked} models: ${error instanceof Error ? error.message : 'unknown error'}`);
    } finally {
      setChecking(false);
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
