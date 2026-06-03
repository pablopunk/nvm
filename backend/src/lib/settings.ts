import { eq, sql } from 'drizzle-orm';
import { db } from '../db/client';
import { appSettings } from '../db/schema';

const ACTIVE_MODEL_KEY = 'active_model';
const FREE_MODEL_KEY = 'free_model';
const ACTIVE_PROVIDER_KEY = 'active_provider';
const DEFAULT_PROVIDER = 'opencode_zen';
const KNOWN_PROVIDERS = new Set(['opencode_zen', 'openrouter']);

export class ModelNotConfiguredError extends Error {
  constructor(public key: 'active_model' | 'free_model') {
    super(`No ${key} configured in app_settings`);
  }
}

async function getSetting(key: string): Promise<string | null> {
  const [row] = await db.select().from(appSettings).where(eq(appSettings.key, key)).limit(1);
  return row?.value ?? null;
}

async function setSetting(key: string, value: string) {
  await db
    .insert(appSettings)
    .values({ key, value })
    .onConflictDoUpdate({ target: appSettings.key, set: { value, updatedAt: sql`now()` } });
}

export async function getActiveModelId(): Promise<string> {
  const v = await getSetting(ACTIVE_MODEL_KEY);
  if (!v) throw new ModelNotConfiguredError('active_model');
  return v;
}

export async function getFreeModelId(): Promise<string> {
  const v = await getSetting(FREE_MODEL_KEY);
  if (!v) throw new ModelNotConfiguredError('free_model');
  return v;
}

export async function setActiveModelId(modelId: string) {
  await setSetting(ACTIVE_MODEL_KEY, modelId);
}

export async function setFreeModelId(modelId: string) {
  await setSetting(FREE_MODEL_KEY, modelId);
}

export async function getActiveProvider(): Promise<string> {
  const v = await getSetting(ACTIVE_PROVIDER_KEY);
  return v && KNOWN_PROVIDERS.has(v) ? v : DEFAULT_PROVIDER;
}

export async function setActiveProvider(provider: string) {
  if (!KNOWN_PROVIDERS.has(provider)) throw new Error(`Unknown provider: ${provider}`);
  await setSetting(ACTIVE_PROVIDER_KEY, provider);
}

export function listKnownProviders(): string[] {
  return [...KNOWN_PROVIDERS];
}

