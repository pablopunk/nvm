import type { CommandAction } from '../palette/model';

const DASHBOARD_LIMIT_KINDS = new Set([
  'insufficient_credits',
  'rate_limited',
  'prompt_too_large',
]);

export function trustedAiLimitAction(
  event: unknown,
  dashboardUrl: string,
): CommandAction | null {
  if (!(event && typeof event === 'object')) return null;
  const candidate = event as { type?: unknown; data?: unknown };
  if (!(candidate.type === 'error' && candidate.data)) return null;
  const data = candidate.data as { kind?: unknown; action?: unknown };
  if (!(data.action && typeof data.action === 'object')) return null;
  const action = data.action as CommandAction;
  if (
    DASHBOARD_LIMIT_KINDS.has(String(data.kind || '')) &&
    action.type === 'openUrl' &&
    action.url === dashboardUrl
  )
    return { type: 'openUrl', title: 'Open Dashboard', url: dashboardUrl };
  if (data.kind === 'unsupported_client' && action.type === 'checkForUpdates')
    return { type: 'checkForUpdates', title: 'Check for Update' };
  return null;
}
