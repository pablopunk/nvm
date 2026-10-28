import { supportsModelThinkingLevel } from './model-capabilities';

const ALLOWED_FIELDS = new Set([
  'model',
  'input',
  'instructions',
  'stream',
  'max_output_tokens',
  'tools',
  'tool_choice',
  'reasoning',
  'include',
  'temperature',
  'top_p',
  'text',
  'parallel_tool_calls',
  'store',
  'prompt_cache_key',
  'prompt_cache_retention',
  'prompt_cache_options',
  'service_tier',
]);
const ALLOWED_INPUT_TYPES = new Set([
  'message',
  'function_call',
  'function_call_output',
  'reasoning',
]);

function record(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

export function validateResponsesRequest(value: unknown): string | null {
  const body = record(value);
  if (!body || Object.keys(body).some((key) => !ALLOWED_FIELDS.has(key)))
    return 'Unsupported Responses request field.';
  if (typeof body.input !== 'string' && !Array.isArray(body.input))
    return 'Responses input must be text or a list of conversation items.';
  if (
    Array.isArray(body.input) &&
    body.input.some((item) => {
      const entry = record(item);
      return (
        !entry ||
        (entry.type !== undefined &&
          (typeof entry.type !== 'string' ||
            !ALLOWED_INPUT_TYPES.has(entry.type)))
      );
    })
  )
    return 'Unsupported Responses conversation item.';
  if (
    body.tools !== undefined &&
    (!Array.isArray(body.tools) ||
      body.tools.some((tool) => record(tool)?.type !== 'function'))
  )
    return 'Only function tools are supported.';
  if (
    body.tool_choice !== undefined &&
    !['auto', 'none', 'required'].includes(String(body.tool_choice)) &&
    record(body.tool_choice)?.type !== 'function'
  )
    return 'Only function tool choices are supported.';
  if (
    body.include !== undefined &&
    (!Array.isArray(body.include) ||
      body.include.some((item) => item !== 'reasoning.encrypted_content'))
  )
    return 'Unsupported Responses include.';
  return null;
}

export function rewriteResponsesRequest(
  bodyText: string,
  routing: { activeModelId: string },
  maxOutputTokens: number,
): string {
  const body = JSON.parse(bodyText);
  body.model = routing.activeModelId;
  body.max_output_tokens = maxOutputTokens;
  body.store = false;
  if (
    typeof body.reasoning?.effort === 'string' &&
    !supportsModelThinkingLevel(routing.activeModelId, body.reasoning.effort)
  ) {
    body.reasoning.effort = 'low';
  }
  delete body.service_tier;
  delete body.prompt_cache_key;
  delete body.prompt_cache_retention;
  delete body.prompt_cache_options;
  return JSON.stringify(body);
}
