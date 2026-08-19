export type ActionExecutionRecord = {
  action: unknown;
  createdAt: number;
};

export function currentActionExecutionRecord(
  executionId: unknown,
  records: Map<string, ActionExecutionRecord>,
  now: number,
  ttlMs: number,
) {
  if (typeof executionId !== 'string') return null;
  const record = records.get(executionId);
  if (!record) return null;
  if (now - record.createdAt <= ttlMs) return record;
  records.delete(executionId);
  return null;
}

export function actionFromExecutionRecord(
  action: unknown,
  records: Map<string, ActionExecutionRecord>,
  options: {
    actionName: (action: Record<string, unknown>) => string;
    now?: number;
    ttlMs: number;
  },
) {
  if (!(action && typeof action === 'object'))
    throw new Error('Untrusted action');
  const input = action as Record<string, unknown>;
  const record = currentActionExecutionRecord(
    input.executionId,
    records,
    options.now ?? Date.now(),
    options.ttlMs,
  );
  if (!record) throw new Error(`Untrusted ${options.actionName(input)} action`);
  const trusted = structuredClone(record.action) as Record<string, unknown>;
  return {
    ...trusted,
    ...(typeof input.traceId === 'string' ? { traceId: input.traceId } : {}),
  };
}
