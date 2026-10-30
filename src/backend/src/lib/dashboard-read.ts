import { Effect, Schedule } from 'effect';

const transientCodes = new Set([
  'ECONNRESET', 'ECONNREFUSED', 'ETIMEDOUT', 'EAI_AGAIN',
  '57P01', '57P02', '57P03', '53300', '08000', '08001', '08003', '08006',
]);

export function isTransientDashboardError(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false;
  const value = error as { code?: unknown; status?: unknown; statusCode?: unknown; cause?: unknown };
  const status = value.status ?? value.statusCode;
  return (typeof value.code === 'string' && transientCodes.has(value.code)) ||
    (typeof status === 'number' && [408, 429, 502, 503, 504].includes(status)) ||
    (value.cause !== error && isTransientDashboardError(value.cause));
}

export function dashboardRead<A>(read: () => Promise<A>): Promise<A> {
  return Effect.runPromise(Effect.tryPromise({ try: read, catch: (error) => error }).pipe(
    Effect.retry({ times: 2, schedule: Schedule.exponential('250 millis'), while: isTransientDashboardError }),
    Effect.timeout('10 seconds'),
  ));
}
