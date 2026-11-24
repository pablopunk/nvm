import type { RouteKind } from '../pricing';

export const MODEL_CAPABILITIES = [
  'thinking_off',
  'reasoning',
  'tool_calls',
  'stream_usage',
  'image_input',
] as const;

export type ModelCapability = (typeof MODEL_CAPABILITIES)[number];

export const CAPABILITIES_FOR_ROUTE_KIND: Record<RouteKind, readonly ModelCapability[]> = {
  fast: ['thinking_off', 'tool_calls', 'stream_usage'],
  smart: ['reasoning', 'image_input', 'tool_calls', 'stream_usage'],
};

export const CAPABILITY_LABELS: Record<ModelCapability, string> = {
  thinking_off: 'answers with thinking turned off',
  reasoning: 'reasons when asked to think',
  tool_calls: 'calls a function tool',
  stream_usage: 'streams a reply that ends with token usage',
  image_input: 'reads an inline image',
};

export function isModelCapability(value: unknown): value is ModelCapability {
  return MODEL_CAPABILITIES.includes(value as ModelCapability);
}

export function capabilitiesForRouteKinds(kinds: Iterable<RouteKind>): ModelCapability[] {
  const needed = new Set<ModelCapability>();
  for (const kind of kinds) for (const capability of CAPABILITIES_FOR_ROUTE_KIND[kind]) needed.add(capability);
  return MODEL_CAPABILITIES.filter((capability) => needed.has(capability));
}
