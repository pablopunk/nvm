import { chatCompletionsReasoningParameter } from './upstream';

export type RouteReasoning = 'route' | 'providerDefault';

export function rewriteChatCompletionsBody(
  bodyText: string,
  routing: {
    activeModelId: string;
    provider: string;
    thinkingLevel: string;
  },
  maxOutputTokens: number,
  reasoning: RouteReasoning = 'route',
): string {
  if (!bodyText) return bodyText;
  const parsed = JSON.parse(bodyText);
  parsed.model = routing.activeModelId;
  // The server owns billing; don't let clients select an unreserved provider tier.
  delete parsed.service_tier;
  if (routing.provider === 'openrouter') {
    parsed.max_tokens = maxOutputTokens;
    delete parsed.max_completion_tokens;
    delete parsed.models;
    delete parsed.route;
    delete parsed.provider;
    delete parsed.plugins;
    delete parsed.web_search_options;
  } else if (parsed.max_completion_tokens !== undefined) {
    parsed.max_completion_tokens = maxOutputTokens;
    delete parsed.max_tokens;
  } else {
    parsed.max_tokens = maxOutputTokens;
  }
  applyRouteThinkingLevel(parsed, routing, reasoning);
  if (parsed.stream === true) {
    parsed.stream_options = {
      ...(parsed.stream_options ?? {}),
      include_usage: true,
    };
  }
  return JSON.stringify(parsed);
}

function applyRouteThinkingLevel(
  parsed: Record<string, unknown>,
  routing: { provider: string; thinkingLevel: string },
  reasoning: RouteReasoning,
) {
  const parameter = chatCompletionsReasoningParameter(routing.provider);
  if (parameter === 'none') return;
  delete parsed.reasoning;
  delete parsed.reasoning_effort;
  if (reasoning === 'providerDefault') return;
  if (parameter === 'openrouter-reasoning') {
    parsed.reasoning = {
      effort: routing.thinkingLevel === 'off' ? 'none' : routing.thinkingLevel,
    };
  } else {
    parsed.reasoning_effort =
      routing.thinkingLevel === 'off' ? 'none' : routing.thinkingLevel;
  }
}
