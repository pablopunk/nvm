import { Param } from 'drizzle-orm';
import type { Database } from '../db/client';
import { appSettings, modelProviders, providers, routeBreakers } from '../db/schema';

export type RouteChainRow = {
  routeSlot: string;
  modelId: string;
  providerId: string;
  providerModelId?: string;
  priority?: number;
};

export type RouteFixture = {
  settings?: Record<string, string>;
  providers?: Record<string, { enabled?: boolean }>;
  chain?: RouteChainRow[];
  breakers?: Array<{ routeSlot: string; provider: string; modelId: string }>;
  failLookups?: Error;
  failChain?: Error;
  failBreakers?: Error;
  failExecute?: Error;
  executeCalls?: { count: number };
};

// Answers only the query shapes that settings.ts issues: a settings row by key, a
// provider's enabled flag by id, and the enabled failover chain by route slot and
// model. Bound values are read from the drizzle condition so the fake exercises the
// same predicates the real queries use. Providers absent from the fixture are enabled.
function collectBoundValues(node: unknown, values: unknown[] = []): unknown[] {
  if (node instanceof Param) values.push(node.value);
  else if (typeof node === 'object' && node !== null) {
    const chunks = (node as { queryChunks?: unknown[] }).queryChunks;
    for (const chunk of chunks ?? []) collectBoundValues(chunk, values);
  }
  return values;
}

function providerEnabled(fixture: RouteFixture, providerId: unknown): boolean {
  return fixture.providers?.[String(providerId)]?.enabled !== false;
}

function chainRowsFor(fixture: RouteFixture, routeSlot: unknown, modelId: unknown) {
  return (fixture.chain ?? [])
    .filter((row) => row.routeSlot === routeSlot && row.modelId === modelId)
    .filter((row) => providerEnabled(fixture, row.providerId))
    .sort((left, right) => (left.priority ?? 0) - (right.priority ?? 0))
    .map((row) => ({ providerId: row.providerId, modelId: row.providerModelId ?? row.modelId }));
}

type SelectQuery = {
  from(source: unknown): SelectQuery;
  innerJoin(): SelectQuery;
  where(condition: unknown): SelectQuery;
  limit(count?: number): Promise<unknown[]>;
  orderBy(...columns: unknown[]): Promise<unknown[]>;
};

function selectQuery(fixture: RouteFixture): SelectQuery {
  if (fixture.failLookups) throw fixture.failLookups;
  let table: unknown;
  let bound: unknown[] = [];
  const rows = (): unknown[] => {
    if (table === appSettings) {
      const [key] = bound as [string];
      const value = fixture.settings?.[key];
      return value === undefined ? [] : [{ key, value }];
    }
    if (table === providers) {
      const [providerId] = bound;
      return [{ enabled: providerEnabled(fixture, providerId) ? 'true' : 'false' }];
    }
    if (table === modelProviders) {
      if (fixture.failChain) throw fixture.failChain;
      const [routeSlot, modelId] = bound;
      return chainRowsFor(fixture, routeSlot, modelId);
    }
    if (table === routeBreakers) {
      if (fixture.failBreakers) throw fixture.failBreakers;
      const [routeSlot, provider, modelId] = bound;
      return (fixture.breakers ?? [])
        .filter((row) => row.routeSlot === routeSlot && row.provider === provider && row.modelId === modelId)
        .map((row) => ({ provider: row.provider }));
    }
    return [];
  };
  const query: SelectQuery = {
    from(source: unknown) {
      table = source;
      return query;
    },
    innerJoin() {
      return query;
    },
    where(condition: unknown) {
      bound = collectBoundValues(condition);
      return query;
    },
    limit: async () => rows(),
    orderBy: async () => rows(),
  };
  return query;
}

export function fakeRouteDb(fixture: RouteFixture): Database {
  return {
    execute: async () => {
      if (fixture.executeCalls) fixture.executeCalls.count += 1;
      if (fixture.failExecute) throw fixture.failExecute;
      return [];
    },
    select: () => selectQuery(fixture),
  } as unknown as Database;
}
