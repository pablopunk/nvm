export const ACTION_EXECUTION_TTL_MS = 30 * 60_000;
const ACTION_EXECUTION_MAX_RECORDS = 2000;

export type ExtensionExecutionRecord = {
  action: any;
  createdAt: number;
  extensionId?: string;
  extensionFile?: string;
};

export type ViewRefreshRecord = {
  entry: any;
  action: any | null;
  viewId?: string;
  mode?: any;
  createdAt: number;
  running?: Promise<any> | null;
  failureCount: number;
  backoffUntil?: number;
};

export function clonePlain(value) {
  if (!value) return value;
  return structuredClone(value);
}

export function withoutExecutionId(value) {
  if (!value || typeof value !== 'object') return value;
  const { executionId, ...rest } = value;
  return rest;
}

export function pruneExecutionRecords(
  store: Map<string, { createdAt: number }>,
) {
  const now = Date.now();
  for (const [id, record] of store) {
    if (now - record.createdAt > ACTION_EXECUTION_TTL_MS) store.delete(id);
  }
  while (store.size > ACTION_EXECUTION_MAX_RECORDS) {
    const oldest = store.keys().next().value;
    if (!oldest) break;
    store.delete(oldest);
  }
}

export function createActionExecutionRecords() {
  const viewActionExecutionRecords = new Map<
    string,
    ExtensionExecutionRecord
  >();
  const rootActionExecutionRecords = new Map<
    string,
    ExtensionExecutionRecord
  >();
  const viewRefreshRecords = new Map<string, ViewRefreshRecord>();

  function pruneViewRefreshRecords() {
    const now = Date.now();
    for (const [id, record] of viewRefreshRecords) {
      if (now - record.createdAt > ACTION_EXECUTION_TTL_MS)
        viewRefreshRecords.delete(id);
    }
    while (viewRefreshRecords.size > ACTION_EXECUTION_MAX_RECORDS) {
      const oldest = viewRefreshRecords.keys().next().value;
      if (!oldest) break;
      viewRefreshRecords.delete(oldest);
    }
  }

  return {
    viewActionExecutionRecords,
    rootActionExecutionRecords,
    viewRefreshRecords,
    pruneViewRefreshRecords,
  };
}

export type ActionExecutionRecords = ReturnType<
  typeof createActionExecutionRecords
>;
