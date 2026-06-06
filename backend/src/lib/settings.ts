import { eq, sql } from 'drizzle-orm';
import { db } from '../db/client';
import { appSettings } from '../db/schema';

const ACTIVE_MODEL_KEY = 'active_model';
const FREE_MODEL_KEY = 'free_model';
const ACTIVE_MODEL_ROUTE_KEY = 'active_model_route';
const FREE_MODEL_ROUTE_KEY = 'free_model_route';
const ACTIVE_PROVIDER_KEY = 'active_provider';
const DEFAULT_PROVIDER = 'opencode_zen';
const KNOWN_PROVIDERS = new Set(['opencode_zen', 'openrouter']);

export type ModelTier = 'free' | 'paid';
export type ModelRoute = { provider: string; modelId: string };

export class ModelNotConfiguredError extends Error {
  constructor(public key: 'active_model' | 'free_model' | 'active_model_route' | 'free_model_route') {
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

export function modelRouteToRef(route: ModelRoute): string {
  return `${route.provider}/${route.modelId}`;
}

export function parseModelRouteRef(ref: string): ModelRoute | null {
  const separator = ref.indexOf('/');
  if (separator <= 0) return null;
  const provider = ref.slice(0, separator);
  const modelId = ref.slice(separator + 1);
  if (!KNOWN_PROVIDERS.has(provider) || !modelId) return null;
  return { provider, modelId };
}

function parseStoredModelRoute(value: string | null): ModelRoute | null {
  if (!value) return null;
  try {
    const parsed = JSON.parse(value) as Partial<ModelRoute>;
    if (parsed.provider && parsed.modelId && KNOWN_PROVIDERS.has(parsed.provider)) {
      return { provider: parsed.provider, modelId: parsed.modelId };
    }
  } catch {
    return parseModelRouteRef(value);
  }
  return null;
}

function modelRouteKey(tier: ModelTier): string {
  return tier === 'free' ? FREE_MODEL_ROUTE_KEY : ACTIVE_MODEL_ROUTE_KEY;
}

async function getLegacyModelId(tier: ModelTier): Promise<string> {
  const key = tier === 'free' ? FREE_MODEL_KEY : ACTIVE_MODEL_KEY;
  const v = await getSetting(key);
  if (!v) throw new ModelNotConfiguredError(tier === 'free' ? 'free_model_route' : 'active_model_route');
  return v;
}

export async function getModelRoute(tier: ModelTier): Promise<ModelRoute> {
  const stored = parseStoredModelRoute(await getSetting(modelRouteKey(tier)));
  if (stored) return stored;
  return { provider: await getActiveProvider(), modelId: await getLegacyModelId(tier) };
}

export async function setModelRoute(tier: ModelTier, route: ModelRoute) {
  if (!KNOWN_PROVIDERS.has(route.provider)) throw new Error(`Unknown provider: ${route.provider}`);
  await setSetting(modelRouteKey(tier), JSON.stringify(route));
}

export async function getActiveModelId(): Promise<string> {
  return (await getModelRoute('paid')).modelId;
}

export async function getFreeModelId(): Promise<string> {
  return (await getModelRoute('free')).modelId;
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

