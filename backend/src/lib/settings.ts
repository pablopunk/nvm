import { eq, sql } from 'drizzle-orm';
import { db } from '../db/client';
import { appSettings } from '../db/schema';
import { MODELS, DEFAULT_MODEL } from './pricing';

const ACTIVE_MODEL_KEY = 'active_model';

export async function getActiveModelId(): Promise<string> {
  const [row] = await db.select().from(appSettings).where(eq(appSettings.key, ACTIVE_MODEL_KEY)).limit(1);
  const v = row?.value;
  if (v && MODELS[v]) return v;
  return DEFAULT_MODEL;
}

export async function setActiveModelId(modelId: string) {
  if (!MODELS[modelId]) throw new Error(`Unknown model: ${modelId}`);
  await db
    .insert(appSettings)
    .values({ key: ACTIVE_MODEL_KEY, value: modelId })
    .onConflictDoUpdate({ target: appSettings.key, set: { value: modelId, updatedAt: sql`now()` } });
}
