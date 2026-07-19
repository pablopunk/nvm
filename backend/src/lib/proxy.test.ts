import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { setDbForTests, resetDbForTests } from '../db/client';
import { requestDedup } from '../db/schema';
import { completeStreamLines, resolveBillableTokens, handleDedup, type StreamUsageAccumulator } from './proxy';

const DUMMY_USER_ID = '11111111-1111-1111-1111-111111111111';
const DUMMY_KEY = 'test-idempotency-key-123';
const STABLE_REQUEST_ID = 'abc123';

function makeRequest(headers: Record<string, string> = {}): Request {
  return new Request('https://api.nvm.fyi/v1/chat/completions', {
    method: 'POST',
    headers: { 'content-type': 'application/json', ...headers },
    body: JSON.stringify({ model: 'test', messages: [{ role: 'user', content: 'hi' }] }),
  });
}

const ctx = {
  user: { id: 'test-user' },
  provider: 'opencode_zen',
  activeModelId: 'gemini-3-flash',
  costRow: { provider: 'opencode_zen', modelId: 'gemini-3-flash', inputUsdPerMtok: 0, outputUsdPerMtok: 0 },
  kind: 'free' as const,
  requestId: 'req_123',
  client: { name: null, version: null, apiVersion: null, platform: null, arch: null },
  estimatedInputTokens: 50,
};

test('completeStreamLines buffers split CRLF and finalizes the trailing line once', function buffersCompleteLines() {
  const acc: StreamUsageAccumulator = { inputTokens: 0, outputTokens: 0, finalized: false };

  assert.deepEqual(completeStreamLines('data: first\r', acc), []);
  assert.deepEqual(completeStreamLines('\ndata: second\r\ntrailing', acc), ['data: first', 'data: second']);
  assert.equal(acc.pendingText, 'trailing');
  assert.deepEqual(completeStreamLines('', acc, true), ['trailing']);
  assert.equal(acc.pendingText, '');
  assert.deepEqual(completeStreamLines('', acc, true), []);
});

test('passes through tokens when output is non-zero', function passesThroughWhenOutputNonZero() {
  const result = resolveBillableTokens(ctx, { inputTokens: 100, outputTokens: 50 }, 200);
  assert.deepEqual(result, { inputTokens: 100, outputTokens: 50 });
});

test('passes through when estimatedInputTokens is 0', function passesThroughZeroEstimate() {
  const noEstimate = { ...ctx, estimatedInputTokens: 0 };
  const result = resolveBillableTokens(noEstimate, { inputTokens: 0, outputTokens: 0 }, 200);
  assert.deepEqual(result, { inputTokens: 0, outputTokens: 0 });
});

test('falls back to estimated input + minimum output on 2xx with zero output', function fallsBackOnMissingUsage() {
  const result = resolveBillableTokens(ctx, { inputTokens: 0, outputTokens: 0 }, 200);
  assert.deepEqual(result, { inputTokens: 50, outputTokens: 1 });
});

test('preserves actual input tokens when only output tokens are missing', function preservesActualInput() {
  const result = resolveBillableTokens(ctx, { inputTokens: 75, outputTokens: 0 }, 200);
  assert.deepEqual(result, { inputTokens: 75, outputTokens: 1 });
});

test('does NOT fall back for non-2xx responses (zero-cost errors)', function keepsZeroCostForErrors() {
  for (const status of [400, 401, 403, 404, 429, 500, 502, 503]) {
    const result = resolveBillableTokens(ctx, { inputTokens: 0, outputTokens: 0 }, status);
    assert.deepEqual(result, { inputTokens: 0, outputTokens: 0 }, `should not fall back for status ${status}`);
  }
});

test('does NOT override non-zero output tokens even on 2xx', function doesNotOverrideNonZeroOutput() {
  const result = resolveBillableTokens(ctx, { inputTokens: 100, outputTokens: 25 }, 200);
  assert.deepEqual(result, { inputTokens: 100, outputTokens: 25 });
});

type DedupRow = {
  id: number;
  userId: string;
  idempotencyKey: string;
  requestHash: string | null;
  status: string;
  responseJson: unknown;
  responseHeaders: Record<string, unknown> | null;
  upstreamStatus: number | null;
  requestId: string | null;
  createdAt: Date;
  completedAt: Date | null;
};

type InsertCall = { table: unknown; values: Record<string, unknown> };
type UpdateCall = { table: unknown; setValues: Record<string, unknown>; whereClause: { keyCol: string; keyVal: unknown } };

function createFakeDedupDb(existingRows: DedupRow[] = []): {
  db: ReturnType<typeof createFakeDbObject>;
  inserts: InsertCall[];
  updates: UpdateCall[];
  rows: DedupRow[];
} {
  const rows: DedupRow[] = existingRows.map((r) => ({ ...r }));
  let nextId = rows.reduce((max, r) => Math.max(max, r.id), 0) + 1;
  const inserts: InsertCall[] = [];
  const updates: UpdateCall[] = [];

  function createFakeDbObject() {
    let lastTable: unknown;
    let lastSetValues: Record<string, unknown> = {};

    function createSelectChain() {
      let whereFilter: (row: DedupRow) => boolean = () => true;
      const chain = {
        from: (table: unknown) => {
          lastTable = table;
          return chain;
        },
        where: (cond: unknown) => {
          whereFilter = (row: DedupRow) => condMatches(row, cond);
          return chain;
        },
        limit: () => {
          const match = rows.filter(whereFilter)[0];
          return Promise.resolve(match ? [match] : []);
        },
        then: (fn: Parameters<Promise<unknown>['then']>[0], rj: Parameters<Promise<unknown>['then']>[1]) =>
          chain.limit().then(fn, rj),
        catch: (fn: Parameters<Promise<unknown>['catch']>[0]) => chain.limit().catch(fn),
      };
      return chain;
    }

    function createInsertChain(table: unknown) {
      let values: Record<string, unknown> = {};
      let onConflictNothing = false;
      const chain: any = {
        values: (next: Record<string, unknown>) => {
          values = next;
          return chain;
        },
        onConflictDoNothing: () => {
          onConflictNothing = true;
          return chain;
        },
        returning: () => {
          const keyConflict = rows.some(
            (r) => r.userId === values.userId && r.idempotencyKey === values.idempotencyKey,
          );
          if (onConflictNothing && keyConflict) {
            return Promise.resolve([]);
          }
          const row: DedupRow = {
            id: nextId++,
            userId: values.userId as string,
            idempotencyKey: values.idempotencyKey as string,
            requestHash: (values.requestHash as string) ?? null,
            status: (values.status as string) ?? 'in_flight',
            responseJson: values.responseJson ?? null,
            responseHeaders: (values.responseHeaders as Record<string, unknown>) ?? null,
            upstreamStatus: (values.upstreamStatus as number) ?? null,
            requestId: (values.requestId as string) ?? null,
            createdAt: (values.createdAt as Date) ?? new Date(),
            completedAt: (values.completedAt as Date) ?? null,
          };
          rows.push(row);
          inserts.push({ table, values: { ...values } });
          return Promise.resolve([row]);
        },
        then: (fn: Parameters<Promise<unknown>['then']>[0], rj: Parameters<Promise<unknown>['then']>[1]) =>
          chain.returning().then(fn, rj),
        catch: (fn: Parameters<Promise<unknown>['catch']>[0]) => chain.returning().catch(fn),
      };
      return chain;
    }

    function createUpdateChain(table: unknown) {
      let storedCond: unknown = null;

      function applyAndReturn(): DedupRow[] {
        const matchedRow = rows.find((r) => rowMatchesAllConditions(r, storedCond));
        if (!matchedRow) return [];
        Object.assign(matchedRow, lastSetValues);
        updates.push({ table, setValues: { ...lastSetValues }, whereClause: { keyCol: 'id', keyVal: matchedRow.id } });
        return [matchedRow];
      }

      const chain: any = {
        set: (next: Record<string, unknown>) => {
          lastSetValues = next;
          return chain;
        },
        where: (cond: unknown) => {
          storedCond = cond;
          return chain;
        },
        returning: () => Promise.resolve(applyAndReturn()),
        then: (fn: Parameters<Promise<unknown>['then']>[0], rj: Parameters<Promise<unknown>['then']>[1]) =>
          Promise.resolve(applyAndReturn()).then(fn, rj),
        catch: (fn: Parameters<Promise<unknown>['catch']>[0]) => Promise.resolve().catch(fn),
      };
      return chain;
    }

    return {
      select: () => createSelectChain(),
      insert: (table: unknown) => createInsertChain(table),
      update: (table: unknown) => createUpdateChain(table),
      transaction: async (fn: (tx: any) => Promise<unknown>) => fn(createFakeDbObject()),
    };
  }

  return { db: createFakeDbObject(), inserts, updates, rows };
}

function condMatches(row: DedupRow, cond: unknown): boolean {
  const clause = cond as { left: { name: string }; right: unknown };
  const col = clause?.left?.name;
  const val = clause?.right;
  if (!col) return true;
  return (row as any)[col] === val;
}

function parseWhereCond(cond: unknown): { col: string; val: unknown } {
  const clause = cond as { left: { name: string }; right: unknown };
  return { col: clause?.left?.name ?? '', val: clause?.right };
}

function extractAndConditions(cond: unknown): Array<{ col: string; val: unknown }> {
  const andCond = cond as { queryChunks?: any[] };
  if (!Array.isArray(andCond.queryChunks)) return [];
  return andCond.queryChunks
    .filter((chunk: any) => chunk && chunk.left && chunk.right !== undefined)
    .map((chunk: any) => ({ col: chunk.left.name, val: chunk.right }));
}

function rowMatchesAllConditions(row: DedupRow, cond: unknown): boolean {
  const eqCol = (cond as any)?.left?.name;
  const eqVal = (cond as any)?.right;
  if (eqCol) return (row as any)[eqCol] === eqVal;

  const andClauses = extractAndConditions(cond);
  if (andClauses.length > 0) {
    return andClauses.every(({ col, val }) => (row as any)[col] === val);
  }

  return true;
}

afterEach(() => {
  resetDbForTests();
});

test('handleDedup inserts new row and returns undefined', async function newDedupRowReturnsUndefined() {
  const { db } = createFakeDedupDb();
  setDbForTests(db as any);
  const result = await handleDedup(DUMMY_KEY, DUMMY_USER_ID, makeRequest(), STABLE_REQUEST_ID);
  assert.equal(result, undefined);
});

test('handleDedup returns 409 when in-flight row exists and is not stale', async function inFlightReturns409() {
  const existingRow: DedupRow = {
    id: 1,
    userId: DUMMY_USER_ID,
    idempotencyKey: DUMMY_KEY,
    requestHash: null,
    status: 'in_flight',
    responseJson: null,
    responseHeaders: null,
    upstreamStatus: null,
    requestId: 'old-req',
    createdAt: new Date(),
    completedAt: null,
  };
  const { db } = createFakeDedupDb([existingRow]);
  setDbForTests(db as any);
  const result = await handleDedup(DUMMY_KEY, DUMMY_USER_ID, makeRequest(), STABLE_REQUEST_ID);
  assert.ok(result instanceof Response);
  assert.equal(result.status, 409);
  const body: any = await result.json();
  assert.equal(body.error.type, 'idempotency_conflict');
});

test('handleDedup reclaims stale in-flight row and returns undefined', async function staleInFlightReclaims() {
  const fiveMinutesAgo = new Date(Date.now() - 6 * 60 * 1000);
  const existingRow: DedupRow = {
    id: 1,
    userId: DUMMY_USER_ID,
    idempotencyKey: DUMMY_KEY,
    requestHash: null,
    status: 'in_flight',
    responseJson: null,
    responseHeaders: null,
    upstreamStatus: null,
    requestId: 'old-req',
    createdAt: fiveMinutesAgo,
    completedAt: null,
  };
  const { db, updates } = createFakeDedupDb([existingRow]);
  setDbForTests(db as any);
  const result = await handleDedup(DUMMY_KEY, DUMMY_USER_ID, makeRequest(), STABLE_REQUEST_ID);
  assert.equal(result, undefined);
  assert.ok(updates.length >= 1);
  assert.equal(updates[0].setValues.status, 'in_flight');
  assert.equal(updates[0].setValues.requestId, STABLE_REQUEST_ID, 'reclaim assigns a new execution identity');
});

test('concurrent stale reclaim permits exactly one winner', async function concurrentStaleReclaimsOneWinner() {
  const fiveMinutesAgo = new Date(Date.now() - 6 * 60 * 1000);
  const existingRow: DedupRow = {
    id: 1,
    userId: DUMMY_USER_ID,
    idempotencyKey: DUMMY_KEY,
    requestHash: null,
    status: 'in_flight',
    responseJson: null,
    responseHeaders: null,
    upstreamStatus: null,
    requestId: 'old-req',
    createdAt: fiveMinutesAgo,
    completedAt: null,
  };
  const { db, updates } = createFakeDedupDb([existingRow]);
  setDbForTests(db as any);

  const result1 = await handleDedup(DUMMY_KEY, DUMMY_USER_ID, makeRequest(), 'winner-req');
  assert.equal(result1, undefined, 'first caller reclaims stale row');
  assert.ok(updates.length >= 1);

  const result2 = await handleDedup(DUMMY_KEY, DUMMY_USER_ID, makeRequest(), 'loser-req');
  assert.ok(result2 instanceof Response);
  assert.equal(result2.status, 409, 'second caller gets idempotency conflict');
});

test('handleDedup replays completed non-streaming response', async function replaysCompletedNonStreaming() {
  const existingRow: DedupRow = {
    id: 1,
    userId: DUMMY_USER_ID,
    idempotencyKey: DUMMY_KEY,
    requestHash: null,
    status: 'completed',
    responseJson: { choices: [{ message: { content: 'hello' } }] },
    responseHeaders: { 'content-type': 'application/json' },
    upstreamStatus: 200,
    requestId: 'old-req',
    createdAt: new Date(),
    completedAt: new Date(),
  };
  const { db } = createFakeDedupDb([existingRow]);
  setDbForTests(db as any);
  const result = await handleDedup(DUMMY_KEY, DUMMY_USER_ID, makeRequest(), STABLE_REQUEST_ID);
  assert.ok(result instanceof Response);
  assert.equal(result.status, 200);
  assert.equal(result.headers.get('x-request-id'), STABLE_REQUEST_ID);
  const body: any = await result.json();
  assert.equal(body.choices[0].message.content, 'hello');
});

test('handleDedup returns 409 for completed streaming response (no responseJson)', async function completedStreamingReturns409() {
  const existingRow: DedupRow = {
    id: 1,
    userId: DUMMY_USER_ID,
    idempotencyKey: DUMMY_KEY,
    requestHash: null,
    status: 'completed',
    responseJson: null,
    responseHeaders: null,
    upstreamStatus: 200,
    requestId: 'old-req',
    createdAt: new Date(),
    completedAt: new Date(),
  };
  const { db } = createFakeDedupDb([existingRow]);
  setDbForTests(db as any);
  const result = await handleDedup(DUMMY_KEY, DUMMY_USER_ID, makeRequest(), STABLE_REQUEST_ID);
  assert.ok(result instanceof Response);
  assert.equal(result.status, 409);
  const body: any = await result.json();
  assert.equal(body.error.type, 'idempotency_conflict');
  assert.equal(body.error.message, 'Request already processed');
});

test('handleDedup reclaims failed row and returns undefined', async function failedRowReclaims() {
  const existingRow: DedupRow = {
    id: 1,
    userId: DUMMY_USER_ID,
    idempotencyKey: DUMMY_KEY,
    requestHash: null,
    status: 'failed',
    responseJson: null,
    responseHeaders: null,
    upstreamStatus: 500,
    requestId: 'old-req',
    createdAt: new Date(),
    completedAt: null,
  };
  const { db, updates } = createFakeDedupDb([existingRow]);
  setDbForTests(db as any);
  const result = await handleDedup(DUMMY_KEY, DUMMY_USER_ID, makeRequest(), STABLE_REQUEST_ID);
  assert.equal(result, undefined);
  assert.ok(updates.length >= 1);
  assert.equal(updates[0].setValues.status, 'in_flight');
  assert.equal(updates[0].setValues.requestId, STABLE_REQUEST_ID, 'failed retry assigns a new execution identity');
});
