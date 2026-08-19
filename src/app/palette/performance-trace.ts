let sequence = 0;

export type RendererPerformanceTrace = {
  traceId: string;
  startedAt: number;
};

export function createRendererPerformanceTrace(
  traceId?: string,
): RendererPerformanceTrace {
  sequence += 1;
  return {
    traceId:
      traceId || `renderer-${Date.now().toString(36)}-${sequence.toString(36)}`,
    startedAt: performance.now(),
  };
}

export function performanceTraceDetail(
  trace: RendererPerformanceTrace,
  detail: Record<string, unknown> = {},
) {
  return { traceId: trace.traceId, ...detail };
}
