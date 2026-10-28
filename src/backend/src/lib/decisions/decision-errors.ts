export type DecisionErrorType =
  | 'invalid_decision_request'
  | 'unsupported_image_route'
  | 'decision_model_unavailable'
  | 'decision_provider_unavailable'
  | 'decision_provider_rate_limited'
  | 'decision_request_rejected'
  | 'decision_provider_response_invalid';

export function decisionErrorResponse(
  status: number,
  type: DecisionErrorType,
  message: string,
  extra: Record<string, unknown> = {},
): Response {
  return Response.json({ error: { type, message, ...extra } }, { status });
}

export function decisionProviderStatusResponse(
  upstreamStatus: number,
): Response {
  if (upstreamStatus === 429)
    return decisionErrorResponse(
      429,
      'decision_provider_rate_limited',
      'The decision model is rate limited. Try again shortly.',
    );
  if ([400, 413, 422].includes(upstreamStatus))
    return decisionErrorResponse(
      400,
      'decision_request_rejected',
      'The decision model rejected this request. Shorten the input or images and try again.',
    );
  return decisionErrorResponse(
    502,
    'decision_provider_unavailable',
    'The decision model is unavailable. Try again shortly.',
  );
}
