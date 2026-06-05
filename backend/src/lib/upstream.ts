import { env } from './env';

export type UpstreamApi = 'openai-completions' | 'anthropic-messages' | 'google-generative-ai';

export class UpstreamConfigError extends Error {}

export function selectApiForModel(provider: string, modelId: string): UpstreamApi {
  if (provider === 'openrouter') return 'openai-completions';
  if (modelId.startsWith('gemini-')) return 'google-generative-ai';
  if (modelId.startsWith('claude-')) return 'anthropic-messages';
  return 'openai-completions';
}

export function getUpstreamConfig(provider: string): { baseUrl: string; apiKey: string } {
  if (provider === 'openrouter') {
    const baseUrl = String(env('OPENROUTER_BASE_URL') ?? 'https://openrouter.ai/api/v1').replace(/\/$/, '');
    const apiKey = String(env('OPENROUTER_API_KEY') ?? '');
    if (!apiKey) throw new UpstreamConfigError('Missing OPENROUTER_API_KEY');
    return { baseUrl, apiKey };
  }
  const baseUrl = String(env('OPENCODE_BASE_URL') ?? 'https://opencode.ai/zen/v1').replace(/\/$/, '');
  const apiKey = String(env('OPENCODE_API_KEY') ?? '');
  if (!apiKey) throw new UpstreamConfigError('Missing OPENCODE_API_KEY');
  return { baseUrl, apiKey };
}
