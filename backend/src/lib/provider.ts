import { createGoogleGenerativeAI } from '@ai-sdk/google';
import { createAnthropic } from '@ai-sdk/anthropic';
import { createOpenAI } from '@ai-sdk/openai';
import { createOpenAICompatible } from '@ai-sdk/openai-compatible';
import type { LanguageModel } from 'ai';

type Upstream = { provider: string; baseURL: string; apiKey: string };

function readUpstream(provider: string): Upstream {
  if (provider === 'openrouter') {
    const baseURL = (import.meta.env.OPENROUTER_BASE_URL ?? 'https://openrouter.ai/api/v1').replace(/\/$/, '');
    const apiKey = import.meta.env.OPENROUTER_API_KEY as string;
    return { provider, baseURL, apiKey };
  }
  const baseURL = (import.meta.env.OPENCODE_BASE_URL ?? 'https://opencode.ai/zen/v1').replace(/\/$/, '');
  const apiKey = import.meta.env.OPENCODE_API_KEY as string;
  return { provider: 'opencode_zen', baseURL, apiKey };
}

export function currentUpstream(provider: string): Upstream {
  return readUpstream(provider);
}

export function modelFor(modelId: string, provider: string): LanguageModel {
  const { baseURL, apiKey, provider: name } = readUpstream(provider);
  if (name === 'openrouter') {
    return createOpenAICompatible({ name: 'openrouter', baseURL, apiKey }).chatModel(modelId);
  }
  if (modelId.startsWith('gemini-')) {
    return createGoogleGenerativeAI({ baseURL, apiKey })(modelId);
  }
  if (modelId.startsWith('claude-')) {
    return createAnthropic({ baseURL, apiKey })(modelId);
  }
  if (modelId.startsWith('gpt-')) {
    return createOpenAI({ baseURL, apiKey })(modelId);
  }
  return createOpenAICompatible({ name: 'opencode', baseURL, apiKey }).chatModel(modelId);
}
