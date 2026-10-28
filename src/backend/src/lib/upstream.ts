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
  const api = apiForCatalogSdk(await catalogSdkForModel(provider, modelId));
  return providerSupportsFormat(provider, api) ? api : 'openai-completions';
}

export async function selectApiForModelChain(primary: { provider: string; modelId: string } | null, modelId: string): Promise<UpstreamApi> {
  return primary?.modelId === modelId ? selectApiForModel(primary.provider, modelId) : 'openai-completions';
}

type ProviderConfig = {
  apiKeyEnv: string;
  baseUrlEnv?: string;
  defaultBaseUrl: string;
  apiFormats: UpstreamApi[];
};

const PROVIDER_CONFIGS: Record<string, ProviderConfig> = {
  opencode_zen: {
    apiKeyEnv: 'OPENCODE_API_KEY',
    baseUrlEnv: 'OPENCODE_BASE_URL',
    defaultBaseUrl: 'https://opencode.ai/zen/v1',
    apiFormats: ['openai-completions', 'openai-responses', 'anthropic-messages', 'google-generative-ai'],
  },
  openrouter: {
    apiKeyEnv: 'OPENROUTER_API_KEY',
    baseUrlEnv: 'OPENROUTER_BASE_URL',
    defaultBaseUrl: 'https://openrouter.ai/api/v1',
    apiFormats: ['openai-completions', 'openai-responses'],
  },
  anthropic: {
    apiKeyEnv: 'ANTHROPIC_API_KEY',
    baseUrlEnv: 'ANTHROPIC_BASE_URL',
    defaultBaseUrl: 'https://api.anthropic.com',
    apiFormats: ['anthropic-messages'],
  },
  openai: {
    apiKeyEnv: 'OPENAI_API_KEY',
    baseUrlEnv: 'OPENAI_BASE_URL',
    defaultBaseUrl: 'https://api.openai.com/v1',
    apiFormats: ['openai-completions', 'openai-responses'],
  },
  google: {
    apiKeyEnv: 'GOOGLE_API_KEY',
    baseUrlEnv: 'GOOGLE_BASE_URL',
    defaultBaseUrl: 'https://generativelanguage.googleapis.com/v1beta',
    apiFormats: ['google-generative-ai'],
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
