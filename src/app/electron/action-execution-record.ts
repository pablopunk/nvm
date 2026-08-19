import { isDeepStrictEqual } from 'node:util';

export function actionMatchesExecutionRecord(
  action: Record<string, unknown>,
  record?: { action: unknown },
) {
  if (!record) return false;
  const { executionId: _executionId, ...payload } = action;
  return isDeepStrictEqual(record.action, payload);
}
