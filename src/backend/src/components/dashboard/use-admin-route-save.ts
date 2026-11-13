import { useState } from 'react';
import type { AdminModelConfig } from '../../lib/admin-model-config';
import { adminMutation } from '../../lib/admin-mutation-request';
import { dashboardQueryClient } from '../../lib/dashboard-query-client';
import type { RouteSaveStatus } from './AdminModelRouteCard';

export const ADMIN_MODEL_CONFIG_URL = '/api/admin/model-config';

/** Shows the choice at once, reverts it if the save fails, then refetches the saved config. */
export function useAdminRouteSave(path: string) {
  const [status, setStatus] = useState<RouteSaveStatus>({ kind: '', message: '' });
  const [saving, setSaving] = useState(false);

  async function save(payload: unknown, applyChoice: (config: AdminModelConfig) => AdminModelConfig) {
    const client = dashboardQueryClient();
    const previous = client.getQueryData<AdminModelConfig>([ADMIN_MODEL_CONFIG_URL]);
    client.setQueryData<AdminModelConfig>([ADMIN_MODEL_CONFIG_URL], (config) => config && applyChoice(config));
    setSaving(true);
    setStatus({ kind: '', message: 'Saving…' });
    try {
      await adminMutation(path, 'PUT', payload);
      setStatus({ kind: 'ok', message: 'Saved' });
    } catch (error) {
      client.setQueryData([ADMIN_MODEL_CONFIG_URL], previous);
      setStatus({ kind: 'err', message: `Could not save: ${error instanceof Error ? error.message : 'unknown error'}` });
    } finally {
      setSaving(false);
      void client.invalidateQueries({ queryKey: [ADMIN_MODEL_CONFIG_URL] });
    }
  }

  return { status, saving, save };
}
