import { createGoogleGenerativeAI } from '@ai-sdk/google';
import { createAnthropic } from '@ai-sdk/anthropic';
import { createOpenAI } from '@ai-sdk/openai';
import { createOpenAICompatible } from '@ai-sdk/openai-compatible';
import type { LanguageModel } from 'ai';

const ZEN_BASE = (import.meta.env.OPENCODE_BASE_URL ?? 'https://opencode.ai/zen/v1').replace(/\/$/, '');
const ZEN_KEY = import.meta.env.OPENCODE_API_KEY as string;

export function modelFor(modelId: string): LanguageModel {
  if (modelId.startsWith('gemini-')) {
    return createGoogleGenerativeAI({ baseURL: ZEN_BASE, apiKey: ZEN_KEY })(modelId);
  }
  if (modelId.startsWith('claude-')) {
    return createAnthropic({ baseURL: ZEN_BASE, apiKey: ZEN_KEY })(modelId);
  }
  if (modelId.startsWith('gpt-')) {
    return createOpenAI({ baseURL: ZEN_BASE, apiKey: ZEN_KEY })(modelId);
  }
  return createOpenAICompatible({ name: 'opencode', baseURL: ZEN_BASE, apiKey: ZEN_KEY }).chatModel(modelId);
}
