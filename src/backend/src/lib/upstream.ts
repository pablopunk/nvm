import { env } from './env';

export type UpstreamApi = 'openai-completions' | 'anthropic-messages' | 'google-generative-ai';

export class UpstreamConfigError extends Error {}

export function selectApiForModel(provider: string, modelId: string): UpstreamApi {
  if (provider === 'openrouter') return 'openai-completions';
  if (modelId.startsWith('gemini-')) return 'google-generative-ai';
  if (modelId.startsWith('claude-')) return 'anthropic-messages';
  return 'openai-completions';
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
    apiFormats: ['openai-completions', 'anthropic-messages', 'google-generative-ai'],
  },
  openrouter: {
    apiKeyEnv: 'OPENROUTER_API_KEY',
    baseUrlEnv: 'OPENROUTER_BASE_URL',
    defaultBaseUrl: 'https://openrouter.ai/api/v1',
    apiFormats: ['openai-completions'],
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
    apiFormats: ['openai-completions'],
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

export function getUpstreamConfig(provider: string): { baseUrl: string; apiKey: string } {
  const cfg = PROVIDER_CONFIGS[provider];
  if (!cfg) throw new UpstreamConfigError(`Unknown provider: ${provider}`);
  const baseUrl = String(env(cfg.baseUrlEnv ?? '') ?? cfg.defaultBaseUrl).replace(/\/$/, '');
  const apiKey = String(env(cfg.apiKeyEnv) ?? '');
  if (!apiKey) throw new UpstreamConfigError(`Missing ${cfg.apiKeyEnv}`);
  return { baseUrl, apiKey };
}
