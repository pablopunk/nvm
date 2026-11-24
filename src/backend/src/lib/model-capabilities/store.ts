import { and, eq, sql } from 'drizzle-orm';
import { modelCapabilityProbes } from '../../db/schema';
import { env } from '../env';
import { isModelCapability, type ModelCapability } from './criteria';

const DAY_MS = 24 * 60 * 60 * 1000;
export const CAPABILITY_REPROBE_AFTER_MS = 7 * DAY_MS;
export const CAPABILITY_VERDICT_MAX_AGE_MS = 14 * DAY_MS;

export type CapabilityVerdict = {
  provider: string;
  modelId: string;
  capability: ModelCapability;
  passed: boolean;
  detail: string | null;
  probedAt: number;
};

export type ModelCapabilityStore = {
  loadProvider(provider: string): Promise<CapabilityVerdict[]>;
  loadModel(provider: string, modelId: string): Promise<CapabilityVerdict[]>;
  save(verdicts: CapabilityVerdict[]): Promise<void>;
};

function verdictFromRow(row: typeof modelCapabilityProbes.$inferSelect): CapabilityVerdict | null {
  if (!isModelCapability(row.capability)) return null;
  return {
    provider: row.provider,
    modelId: row.modelId,
    capability: row.capability,
    passed: row.passed,
    detail: row.detail,
    probedAt: Number(row.probedAt),
  };
}

function knownVerdicts(rows: Array<typeof modelCapabilityProbes.$inferSelect>): CapabilityVerdict[] {
  return rows.map(verdictFromRow).filter((verdict): verdict is CapabilityVerdict => verdict !== null);
}

function databaseModelCapabilityStore(): ModelCapabilityStore {
  return {
    async loadProvider(provider) {
      if (!env('DATABASE_URL')) return [];
      const { getDb } = await import('../../db/client');
      return knownVerdicts(
        await getDb().select().from(modelCapabilityProbes).where(eq(modelCapabilityProbes.provider, provider)),
      );
    },
    async loadModel(provider, modelId) {
      if (!env('DATABASE_URL')) return [];
      const { getDb } = await import('../../db/client');
      return knownVerdicts(
        await getDb()
          .select()
          .from(modelCapabilityProbes)
          .where(and(eq(modelCapabilityProbes.provider, provider), eq(modelCapabilityProbes.modelId, modelId))),
      );
    },
    async save(verdicts) {
      if (!env('DATABASE_URL') || verdicts.length === 0) return;
      const { getDb } = await import('../../db/client');
      await getDb()
        .insert(modelCapabilityProbes)
        .values(verdicts)
        .onConflictDoUpdate({
          target: [modelCapabilityProbes.provider, modelCapabilityProbes.modelId, modelCapabilityProbes.capability],
          set: {
            passed: sql`excluded.passed`,
            detail: sql`excluded.detail`,
            probedAt: sql`excluded.probed_at`,
          },
        });
    },
  };
}

export function memoryModelCapabilityStore(): ModelCapabilityStore {
  const verdicts = new Map<string, CapabilityVerdict>();
  const keyOf = (verdict: Pick<CapabilityVerdict, 'provider' | 'modelId' | 'capability'>) =>
    `${verdict.provider}\u0000${verdict.modelId}\u0000${verdict.capability}`;
  return {
    async loadProvider(provider) {
      return [...verdicts.values()].filter((verdict) => verdict.provider === provider);
    },
    async loadModel(provider, modelId) {
      return [...verdicts.values()].filter((verdict) => verdict.provider === provider && verdict.modelId === modelId);
    },
    async save(next) {
      for (const verdict of next) verdicts.set(keyOf(verdict), verdict);
    },
  };
}

let activeStore: ModelCapabilityStore = databaseModelCapabilityStore();

export function activeModelCapabilityStore(): ModelCapabilityStore {
  return activeStore;
}

export function useMemoryModelCapabilityStoreForTests(): ModelCapabilityStore {
  activeStore = memoryModelCapabilityStore();
  return activeStore;
}

export function resetModelCapabilityStoreForTests() {
  activeStore = databaseModelCapabilityStore();
}

export function isVerdictFresh(verdict: CapabilityVerdict, now = Date.now()): boolean {
  return now - verdict.probedAt < CAPABILITY_VERDICT_MAX_AGE_MS;
}

export function needsReprobe(verdict: CapabilityVerdict | undefined, now = Date.now()): boolean {
  return !verdict || now - verdict.probedAt >= CAPABILITY_REPROBE_AFTER_MS;
}

export function passedCapabilitiesByModel(
  verdicts: CapabilityVerdict[],
  now = Date.now(),
): Map<string, Set<ModelCapability>> {
  const passed = new Map<string, Set<ModelCapability>>();
  for (const verdict of verdicts) {
    if (!verdict.passed || !isVerdictFresh(verdict, now)) continue;
    const capabilities = passed.get(verdict.modelId) ?? new Set<ModelCapability>();
    capabilities.add(verdict.capability);
    passed.set(verdict.modelId, capabilities);
  }
  return passed;
}
