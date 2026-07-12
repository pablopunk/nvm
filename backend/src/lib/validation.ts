import type { z, ZodSchema } from 'zod';
import { env } from './env';
import { log } from './log';

type ValidationMode = 'strict' | 'warn' | 'off';

type SafeJsonOk<T> = { ok: true; data: T };
type SafeJsonErr = { ok: false; error: { error: { type: 'invalid_request'; message: string; issues?: z.ZodIssue[] } } };

export type SafeJsonResult<T> = SafeJsonOk<T> | SafeJsonErr;

function validationMode(): ValidationMode {
  const raw = env('NEVERMIND_VALIDATION_MODE');
  if (raw === 'off') return 'off';
  if (raw === 'warn') return 'warn';
  return 'strict';
}

export async function safeJsonBody<T>(request: Request, schema: ZodSchema<T>): Promise<SafeJsonResult<T>> {
  const mode = validationMode();

  let raw: unknown;
  try {
    raw = await request.json();
  } catch {
    if (mode === 'strict') {
      return { ok: false, error: { error: { type: 'invalid_request', message: 'Request body is not valid JSON' } } };
    }
    raw = {};
  }

  if (mode === 'off') return { ok: true, data: raw as T };

  const result = schema.safeParse(raw);
  if (result.success) return { ok: true, data: result.data };

  if (mode === 'warn') {
    log.warn('validation_rejected', { route: new URL(request.url).pathname, issues: result.error.issues });
    return { ok: true, data: raw as T };
  }

  return {
    ok: false,
    error: {
      error: {
        type: 'invalid_request',
        message: 'Request body validation failed',
        issues: result.error.issues,
      },
    },
  };
}
