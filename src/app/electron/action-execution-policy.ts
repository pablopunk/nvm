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

export function nativeActionFromExecutionRecord(
  action: unknown,
  rootRecords: Map<string, ActionExecutionRecord>,
  options: { now?: number; ttlMs: number },
) {
  if (!(action && typeof action === 'object'))
    throw new Error('Untrusted nativeAction action');
  const input = action as Record<string, unknown>;
  if (input.type !== 'nativeAction')
    throw new Error('Untrusted nativeAction action');
  const traceId = typeof input.traceId === 'string' ? input.traceId : undefined;
  const nativeInput = input.nativeAction;
  if (!(nativeInput && typeof nativeInput === 'object'))
    throw new Error('Untrusted nativeAction action');
  const nativeAction = actionFromExecutionRecord(
    { ...nativeInput, ...(traceId ? { traceId } : {}) },
    rootRecords,
    {
      actionName: (nested) => String(nested.kind || 'root'),
      now: options.now,
      ttlMs: options.ttlMs,
    },
  );
  return {
    type: 'nativeAction',
    title: input.title,
    nativeAction,
    ...(traceId ? { traceId } : {}),
  };
}
