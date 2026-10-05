import fs from 'node:fs/promises';
import path from 'node:path';
import {
  type DiagnosticRecord,
  diagnosticOperation,
  isDiagnosticId,
  isRequestId,
} from '../shared/diagnostics';

export const DIAGNOSTIC_FILE_NAME = 'nevermind-diagnostics.json';
const MAX_BYTES = 1024 * 1024;
const MAX_RECORDS = 100;
const MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000;
const STAGES = new Set([
  'dispatch',
  'invoke',
  'load',
  'render',
  'permission',
  'request',
  'stream',
  'validate',
  'save',
  'install',
  'complete',
  'unknown',
]);
const OUTCOMES = new Set([
  'success',
  'failed',
  'cancelled',
  'blocked',
  'timed_out',
  'interrupted',
  'unknown',
]);

function safeRecord(value: unknown): DiagnosticRecord | undefined {
  if (!value || typeof value !== 'object') return;
  const record = value as DiagnosticRecord;
  if (!isDiagnosticId(record.reference) || !isDiagnosticId(record.bootId))
    return;
  if (
    !Number.isFinite(Date.parse(record.timestamp)) ||
    Date.parse(record.timestamp) < Date.now() - MAX_AGE_MS
  )
    return;
  if (
    typeof record.build !== 'string' ||
    !/^[a-zA-Z0-9@._+-]{1,120}$/.test(record.build)
  )
    return;
  return {
    reference: record.reference,
    timestamp: new Date(record.timestamp).toISOString(),
    bootId: record.bootId,
    build: record.build,
    operation: diagnosticOperation(record.operation),
    stage: STAGES.has(record.stage) ? record.stage : 'unknown',
    outcome: OUTCOMES.has(record.outcome) ? record.outcome : 'unknown',
    process: record.process === 'renderer' ? 'renderer' : 'main',
    reporting:
      record.reporting === 'capture_requested' || record.reporting === 'dropped'
        ? record.reporting
        : 'local_only',
    ...(Number.isSafeInteger(record.windowId) && Number(record.windowId) > 0
      ? { windowId: record.windowId }
      : {}),
    ...(isDiagnosticId(record.journeyId)
      ? { journeyId: record.journeyId }
      : {}),
    ...(isDiagnosticId(record.actionId) ? { actionId: record.actionId } : {}),
    ...(isDiagnosticId(record.eventId) ? { eventId: record.eventId } : {}),
    ...(isRequestId(record.requestId) ? { requestId: record.requestId } : {}),
    ...(typeof record.projectId === 'string' &&
    /^\d{1,24}$/.test(record.projectId)
      ? { projectId: record.projectId }
      : {}),
  };
}

export function createDiagnosticStore(filePath: string) {
  let records: DiagnosticRecord[] = [];
  let writes = Promise.resolve();
  let loaded = false;

  function recent() {
    records = records
      .filter(
        (record) => Date.parse(record.timestamp) >= Date.now() - MAX_AGE_MS,
      )
      .slice(0, MAX_RECORDS);
    return records.map((record) => ({ ...record }));
  }

  async function load() {
    if (loaded) return;
    loaded = true;
    try {
      const handle = await fs.open(filePath, 'r');
      try {
        if ((await handle.stat()).size > MAX_BYTES) return;
        const data = JSON.parse(await handle.readFile('utf8'));
        if (data.version !== 1 || !Array.isArray(data.records)) return;
        const existing = data.records
          .slice(0, MAX_RECORDS)
          .map(safeRecord)
          .filter(Boolean) as DiagnosticRecord[];
        records = [
          ...records,
          ...existing.filter(
            (record) =>
              !records.some(
                (current) => current.reference === record.reference,
              ),
          ),
        ].slice(0, MAX_RECORDS);
      } finally {
        await handle.close();
      }
    } catch {}
  }

  function put(record: DiagnosticRecord) {
    const safe = safeRecord(record);
    if (!safe) return;
    records = [
      safe,
      ...recent().filter((current) => current.reference !== safe.reference),
    ].slice(0, MAX_RECORDS);
    writes = writes
      .then(async function persistDiagnostics() {
        const data = JSON.stringify({ version: 1, records: recent() });
        if (Buffer.byteLength(data) > MAX_BYTES) return;
        await fs.mkdir(path.dirname(filePath), { recursive: true });
        const temporaryPath = `${filePath}.pending`;
        await fs.writeFile(temporaryPath, data, { mode: 0o600 });
        await fs.rename(temporaryPath, filePath);
      })
      .catch(() => {});
  }

  return { load, put, recent, flush: () => writes };
}
