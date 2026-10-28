import { env } from './env';
import { catalogSdkForModel, type CatalogSdk } from './pricing';

export type UpstreamApi = 'openai-completions' | 'openai-responses' | 'anthropic-messages' | 'google-generative-ai';

export class UpstreamConfigError extends Error {}

const API_FOR_CATALOG_SDK: Record<string, UpstreamApi> = {
  '@ai-sdk/openai': 'openai-responses',
  '@ai-sdk/anthropic': 'anthropic-messages',
  '@ai-sdk/google': 'google-generative-ai',
};

function apiForCatalogSdk(sdk: CatalogSdk): UpstreamApi {
  if (sdk.shape === 'responses') return 'openai-responses';
  return API_FOR_CATALOG_SDK[sdk.npm ?? ''] ?? 'openai-completions';
}

export async function selectApiForModel(provider: string, modelId: string): Promise<UpstreamApi> {
  return apiForCatalogSdk(await catalogSdkForModel(provider, modelId));
}

export async function selectApiForModelChain(primary: { provider: string; modelId: string } | null, modelId: string): Promise<UpstreamApi> {
  return primary?.modelId === modelId ? selectApiForModel(primary.provider, modelId) : 'openai-completions';
}

type UpstreamAuthHeader = 'authorization' | 'x-api-key' | 'x-goog-api-key';

type UpstreamCatalogListKey = 'data' | 'models';

// `catalog` means the model catalog rejects an invalid credential, so a 2xx sent
// with the configured key proves that key is accepted. It does not mean anonymous
// callers are refused: OpenCode serves its catalog anonymously for legacy keyless
// access, so only a 2xx with the key sent says anything about the key. `key` names
// an authenticated, non-billable endpoint for providers whose catalog answers any
// bearer value, such as OpenRouter's `/key`.
export type UpstreamVerification = { kind: 'catalog' } | { kind: 'key'; path: string };

type ProviderConfig = {
  apiKeyEnv: string;
  baseUrlEnv?: string;
  defaultBaseUrl: string;
  apiFormats: UpstreamApi[];
  authHeader: UpstreamAuthHeader;
  catalogPath?: string;
  catalogListKey?: UpstreamCatalogListKey;
  extraHeaders?: Record<string, string>;
  verification: UpstreamVerification;
};

const PROVIDER_CONFIGS: Record<string, ProviderConfig> = {
  opencode_zen: {
    apiKeyEnv: 'OPENCODE_API_KEY',
    baseUrlEnv: 'OPENCODE_BASE_URL',
    defaultBaseUrl: 'https://opencode.ai/zen/v1',
    apiFormats: ['openai-completions', 'openai-responses', 'anthropic-messages', 'google-generative-ai'],
    authHeader: 'authorization',
    verification: { kind: 'catalog' },
  },
  openrouter: {
    apiKeyEnv: 'OPENROUTER_API_KEY',
    baseUrlEnv: 'OPENROUTER_BASE_URL',
    defaultBaseUrl: 'https://openrouter.ai/api/v1',
    apiFormats: ['openai-completions', 'openai-responses'],
    authHeader: 'authorization',
    verification: { kind: 'key', path: '/key' },
  },
  anthropic: {
    apiKeyEnv: 'ANTHROPIC_API_KEY',
    baseUrlEnv: 'ANTHROPIC_BASE_URL',
    defaultBaseUrl: 'https://api.anthropic.com',
    apiFormats: ['anthropic-messages'],
    authHeader: 'x-api-key',
    catalogPath: '/v1/models',
    extraHeaders: { 'anthropic-version': '2023-06-01' },
    verification: { kind: 'catalog' },
  },
  openai: {
    apiKeyEnv: 'OPENAI_API_KEY',
    baseUrlEnv: 'OPENAI_BASE_URL',
    defaultBaseUrl: 'https://api.openai.com/v1',
    apiFormats: ['openai-completions', 'openai-responses'],
    authHeader: 'authorization',
    verification: { kind: 'catalog' },
  },
  google: {
    apiKeyEnv: 'GOOGLE_API_KEY',
    baseUrlEnv: 'GOOGLE_BASE_URL',
    defaultBaseUrl: 'https://generativelanguage.googleapis.com/v1beta',
    apiFormats: ['google-generative-ai'],
    authHeader: 'x-goog-api-key',
    catalogListKey: 'models',
    verification: { kind: 'catalog' },
  },
};

export function providerSupportsFormat(provider: string, format: UpstreamApi): boolean {
  const cfg = PROVIDER_CONFIGS[provider];
  return cfg ? cfg.apiFormats.includes(format) : false;
}

export function getUpstreamBaseUrl(provider: string): string {
  const cfg = PROVIDER_CONFIGS[provider];
  if (!cfg) throw new UpstreamConfigError(`Unknown provider: ${provider}`);
  return String(env(cfg.baseUrlEnv ?? '') ?? cfg.defaultBaseUrl).replace(/\/$/, '');
}

export function getUpstreamConfig(provider: string): { baseUrl: string; apiKey: string } {
  const cfg = PROVIDER_CONFIGS[provider];
  if (!cfg) throw new UpstreamConfigError(`Unknown provider: ${provider}`);
  const baseUrl = getUpstreamBaseUrl(provider);
  const apiKey = String(env(cfg.apiKeyEnv) ?? '');
  if (!apiKey) throw new UpstreamConfigError(`Missing ${cfg.apiKeyEnv}`);
  return { baseUrl, apiKey };
}

export type UpstreamProbeSpec = {
  baseUrl: string;
  catalogPath: string;
  catalogListKey: UpstreamCatalogListKey;
  headers: Record<string, string>;
  verification: UpstreamVerification;
};

export function getUpstreamProbeSpec(provider: string): UpstreamProbeSpec {
  const cfg = PROVIDER_CONFIGS[provider];
  if (!cfg) throw new UpstreamConfigError(`Unknown provider: ${provider}`);
  const { baseUrl, apiKey } = getUpstreamConfig(provider);
  const authValue = cfg.authHeader === 'authorization' ? `Bearer ${apiKey}` : apiKey;
  return {
    baseUrl,
    catalogPath: cfg.catalogPath ?? '/models',
    catalogListKey: cfg.catalogListKey ?? 'data',
    headers: { ...cfg.extraHeaders, [cfg.authHeader]: authValue },
    verification: cfg.verification,
  };
}
