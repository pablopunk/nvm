import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { resetDbForTests, setDbForTests } from '../db/client';
import { adminMutationResults } from '../db/schema';
import {
  adminMutationKeyFromRequest,
  adminMutationPayloadHash,
  executeAdminMutation,
  lookupAdminMutationResult,
} from './admin-mutations';

afterEach(() => resetDbForTests());

function createMutationDatabase() {
  const rows = new Map<string, Record<string, unknown>>();
  let nextId = 1;

  function mutationKey(row: Record<string, unknown>) {
    return `${row.actorUserId}:${row.idempotencyKey}`;
  }

  function selectBuilder() {
    let table: unknown = null;
    const currentRows = () =>
      table === adminMutationResults
        ? [...rows.values()].map((row) => ({ ...row }))
        : [];
    const builder = {
      from(value: unknown) {
        table = value;
        return builder;
      },
      where() {
        return builder;
      },
      for() {
        return Promise.resolve(currentRows());
      },
      limit() {
        return Promise.resolve(currentRows());
      },
    };
    return builder;
  }

  function createTransaction() {
    return {
      insert(table: unknown) {
        let values: Record<string, unknown> = {};
        const builder = {
          values(input: Record<string, unknown>) {
            values = input;
            return builder;
          },
          onConflictDoNothing() {
            return builder;
          },
          async returning() {
            if (table !== adminMutationResults) return [];
            const key = mutationKey(values);
            if (rows.has(key)) return [];
            const row = { ...values, id: nextId++ };
            rows.set(key, row);
            return [{ id: row.id }];
          },
        };
        return builder;
      },
      select: selectBuilder,
      update(table: unknown) {
        let values: Record<string, unknown> = {};
        return {
          set(input: Record<string, unknown>) {
            values = input;
            return {
              async where() {
                if (table !== adminMutationResults) return;
                const row = [...rows.values()].at(-1);
                if (row) Object.assign(row, values);
              },
            };
          },
        };
      },
    };
  }

  const database = {
    rows,
    select: selectBuilder,
    async transaction<T>(
      callback: (
        transaction: ReturnType<typeof createTransaction>,
      ) => Promise<T>,
    ) {
      const before = new Map(
        [...rows.entries()].map(([key, row]) => [key, { ...row }]),
      );
      try {
        return await callback(createTransaction());
      } catch (error) {
        rows.clear();
        for (const [key, row] of before) rows.set(key, row);
        throw error;
      }
    },
  };
  setDbForTests(database as any);
  return database;
}

test('admin mutation payload hashes ignore object key order but preserve array order', () => {
  assert.equal(
    adminMutationPayloadHash({ userId: 'user-1', delta: -5, kind: 'free' }),
    adminMutationPayloadHash({ kind: 'free', delta: -5, userId: 'user-1' }),
  );
  assert.notEqual(
    adminMutationPayloadHash({ ids: ['a', 'b'] }),
    adminMutationPayloadHash({ ids: ['b', 'a'] }),
  );
});

test('admin mutation payload hashes are defined for JSON-null-like payloads', () => {
  assert.equal(
    adminMutationPayloadHash(undefined),
    adminMutationPayloadHash(null),
  );
  assert.match(
    adminMutationPayloadHash({ value: undefined }),
    /^[a-f0-9]{64}$/,
  );
});

test('admin mutation keys accept bounded UUIDs and reject malformed values', () => {
  const accepted = new Request('https://www.nvm.fyi/api/admin/credits', {
    method: 'POST',
    headers: { 'Idempotency-Key': '11111111-1111-4111-8111-111111111111' },
  });
  const malformed = new Request('https://www.nvm.fyi/api/admin/credits', {
    method: 'POST',
    headers: { 'Idempotency-Key': 'not-a-uuid' },
  });

  assert.equal(
    adminMutationKeyFromRequest(accepted),
    '11111111-1111-4111-8111-111111111111',
  );
  assert.equal(adminMutationKeyFromRequest(malformed), null);
});

test('admin mutation retries return the saved result and reject changed payloads', async () => {
  createMutationDatabase();
  const input = {
    actorUserId: 'admin-1',
    idempotencyKey: '11111111-1111-4111-8111-111111111111',
    payload: { userId: 'user-1', delta: 25 },
  };
  let executions = 0;
  const first = await executeAdminMutation({
    ...input,
    execute: async () => {
      executions += 1;
      return { body: { ok: true, balance: 25 } };
    },
  });
  const replay = await executeAdminMutation({
    ...input,
    execute: async () => {
      executions += 1;
      return { body: { ok: false } };
    },
  });
  const conflict = await lookupAdminMutationResult({
    ...input,
    payload: { userId: 'user-1', delta: 50 },
  });

  assert.equal(first.status, 200);
  assert.deepEqual(first.body, { ok: true, balance: 25 });
  assert.equal(replay.replayed, true);
  assert.deepEqual(replay.body, first.body);
  assert.equal(conflict?.status, 409);
  assert.equal(conflict?.conflict, true);
  assert.equal(executions, 1);
});

test('admin mutation execution failures roll back the idempotency row', async () => {
  const database = createMutationDatabase();
  await assert.rejects(
    executeAdminMutation({
      actorUserId: 'admin-1',
      idempotencyKey: '22222222-2222-4222-8222-222222222222',
      payload: { action: 'role-change' },
      execute: async () => {
        throw new Error('audit insert failed');
      },
    }),
    /audit insert failed/,
  );
  assert.equal(database.rows.size, 0);
});
