import { eq, sql, and } from 'drizzle-orm';
import { db } from '../db/client';
import { appSettings, modelProviders, providers } from '../db/schema';
import { env } from './env';

export const SIGNUPS_ENABLED_KEY = 'signups_enabled';

export class SignupsPolicyError extends Error {
  constructor(message: string, options?: { cause?: unknown }) {
    super(message, options);
    this.name = 'SignupsPolicyError';
  }
}

const ACTIVE_MODEL_KEY = 'active_model';
const FREE_MODEL_KEY = 'free_model';
const ACTIVE_MODEL_ROUTE_KEY = 'active_model_route';
const FREE_MODEL_ROUTE_KEY = 'free_model_route';
const SMART_MODEL_ROUTE_KEY = 'smart_model_route';
const FAST_MODEL_ROUTE_KEY = 'fast_model_route';
const ACTIVE_PROVIDER_KEY = 'active_provider';
const DEFAULT_PROVIDER = 'opencode_zen';
const KNOWN_PROVIDERS = new Set(['opencode_zen', 'openrouter', 'anthropic', 'openai', 'google']);

export type ModelTier = 'free' | 'paid';
export type ExtensionAiModelRole = 'smart' | 'fast';
export type ModelRouteSlot = ModelTier | ExtensionAiModelRole;
export type ModelRoute = { provider: string; modelId: string };

export class ModelNotConfiguredError extends Error {
  constructor(public key: 'active_model' | 'free_model' | 'active_model_route' | 'free_model_route' | 'smart_model_route' | 'fast_model_route') {
    super(`No ${key} configured in app_settings`);
  }
}

async function getSetting(key: string): Promise<string | null> {
  const [row] = await db.select().from(appSettings).where(eq(appSettings.key, key)).limit(1);
  return row?.value ?? null;
}

/**
 * Release A compatibility reader. A persisted setting is authoritative; the
 * legacy deployment flag is consulted only while the row is absent.
 */
export async function getSignupsEnabled(): Promise<boolean> {
  let row: { value: string } | undefined;
  try {
    [row] = await db
      .select({ value: appSettings.value })
      .from(appSettings)
      .where(eq(appSettings.key, SIGNUPS_ENABLED_KEY))
      .limit(1);
  } catch (error) {
    throw new SignupsPolicyError('Sign-up policy is unavailable', { cause: error });
  }

  if (!row) return env('INVITE_GATE_ENABLED') !== 'true';
  if (row.value === 'true') return true;
  if (row.value === 'false') return false;
  throw new SignupsPolicyError('Sign-up policy is malformed');
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

function modelRouteKey(slot: ModelRouteSlot): string {
  if (slot === 'smart') return SMART_MODEL_ROUTE_KEY;
  if (slot === 'fast') return FAST_MODEL_ROUTE_KEY;
  return slot === 'free' ? FREE_MODEL_ROUTE_KEY : ACTIVE_MODEL_ROUTE_KEY;
}

function fallbackRouteSlot(slot: ModelRouteSlot): ModelTier | null {
  if (slot === 'smart') return 'paid';
  if (slot === 'fast') return 'free';
  return null;
}

async function getLegacyModelId(tier: ModelTier): Promise<string> {
  const key = tier === 'free' ? FREE_MODEL_KEY : ACTIVE_MODEL_KEY;
  const v = await getSetting(key);
  if (!v) throw new ModelNotConfiguredError(tier === 'free' ? 'free_model_route' : 'active_model_route');
  return v;
}

export async function getModelRoute(slot: ModelRouteSlot): Promise<ModelRoute> {
  const stored = parseStoredModelRoute(await getSetting(modelRouteKey(slot)));
  if (stored) return stored;
  const fallback = fallbackRouteSlot(slot);
  if (fallback) return getModelRoute(fallback);
  return { provider: await getActiveProvider(), modelId: await getLegacyModelId(slot as ModelTier) };
}

export async function setModelRoute(slot: ModelRouteSlot, route: ModelRoute) {
  if (!KNOWN_PROVIDERS.has(route.provider)) throw new Error(`Unknown provider: ${route.provider}`);
  await setSetting(modelRouteKey(slot), JSON.stringify(route));
}

export function parseExtensionAiModelRole(value: string | null | undefined): ExtensionAiModelRole | null {
  return value === 'smart' || value === 'fast' ? value : null;
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

// ── Provider chain (failover) ──

export async function getModelProviderChain(slot: ModelRouteSlot, modelId: string): Promise<string[]> {
  const rows = await db
    .select({ providerId: modelProviders.providerId })
    .from(modelProviders)
    .innerJoin(providers, eq(modelProviders.providerId, providers.id))
    .where(
      and(
        eq(modelProviders.routeSlot, slot),
        eq(modelProviders.modelId, modelId),
        eq(providers.enabled, 'true'),
      ),
    )
    .orderBy(modelProviders.priority);

  return rows.map((r) => r.providerId);
}

export async function setModelProviderChain(
  slot: ModelRouteSlot,
  modelId: string,
  providerIds: string[],
) {
  await db.transaction(async (tx) => {
    await tx
      .delete(modelProviders)
      .where(
        and(
          eq(modelProviders.routeSlot, slot),
          eq(modelProviders.modelId, modelId),
        ),
      );

    if (providerIds.length === 0) return;

    const values = providerIds.map((providerId, i) => ({
      routeSlot: slot,
      modelId,
      providerId,
      priority: i,
    }));

    await tx.insert(modelProviders).values(values);
  });
}

export async function listEnabledProviders() {
  return db
    .select()
    .from(providers)
    .where(eq(providers.enabled, 'true'))
    .orderBy(providers.priority);
}

export async function listAllProviders() {
  return db.select().from(providers).orderBy(providers.priority);
}

export async function updateProvider(
  providerId: string,
  updates: { enabled?: boolean; priority?: number },
) {
  const set: Record<string, unknown> = { updatedAt: sql`now()` };
  if (updates.enabled !== undefined) set.enabled = updates.enabled ? 'true' : 'false';
  if (updates.priority !== undefined) set.priority = updates.priority;
  await db.update(providers).set(set).where(eq(providers.id, providerId));
}
