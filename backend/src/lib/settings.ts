import { eq, sql } from 'drizzle-orm';
import { db } from '../db/client';
import { appSettings } from '../db/schema';
import { MODELS, DEFAULT_MODEL } from './pricing';

const ACTIVE_MODEL_KEY = 'active_model';
const FREE_MODEL_KEY = 'free_model';

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
  return v && MODELS[v] ? v : DEFAULT_MODEL;
}

export async function getFreeModelId(): Promise<string> {
  const v = await getSetting(FREE_MODEL_KEY);
  if (v && MODELS[v]) return v;
  return getActiveModelId();
}

export async function setActiveModelId(modelId: string) {
  if (!MODELS[modelId]) throw new Error(`Unknown model: ${modelId}`);
  await setSetting(ACTIVE_MODEL_KEY, modelId);
}

export async function setFreeModelId(modelId: string) {
  if (!MODELS[modelId]) throw new Error(`Unknown model: ${modelId}`);
  await setSetting(FREE_MODEL_KEY, modelId);
}
