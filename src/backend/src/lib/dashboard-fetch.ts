import { Effect, Schedule } from 'effect';

class DashboardRequestError extends Error {
  constructor(message: string, readonly retryable: boolean) {
    super(message);
  }
}

export function dashboardFetch<T>(url: string): Promise<T> {
  const request = Effect.tryPromise({
    try: async (signal) => {
      let response: Response;
      try {
        response = await fetch(url, { signal, cache: 'no-store' });
      } catch (error) {
        if (signal.aborted) throw error;
        throw new DashboardRequestError('Network error', true);
      }
      if (!response.ok) {
        const body = await response.json().catch(() => null);
        throw new DashboardRequestError(
          body?.error?.message || response.statusText || 'Request failed',
          [408, 429, 500, 502, 503, 504].includes(response.status),
        );
      }
      return response.json() as Promise<T>;
    },
    catch: (error) => error instanceof DashboardRequestError ? error : new DashboardRequestError('Request failed', false),
  });
  return Effect.runPromise(request.pipe(
    Effect.retry({ times: 2, schedule: Schedule.exponential('250 millis'), while: (error) => error.retryable }),
    Effect.timeout('10 seconds'),
  ));
}
