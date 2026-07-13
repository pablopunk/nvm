type LogLevel = 'debug' | 'info' | 'warn' | 'error';

type LogFields = {
  request_id?: string;
  user_id?: string;
  route?: string;
  status?: number;
  latency_ms?: number;
  model?: string;
  provider?: string;
  kind?: 'free' | 'paid';
  error?: unknown;
  [key: string]: unknown;
};

const SENSITIVE_AUTH_QUERY_KEYS = new Set(['code', 'state', 'intent', 'grant', 'sealedSession', 'return_to']);

export function redactAuthUrl(input: URL | string) {
  const url = new URL(input.toString());
  for (const key of SENSITIVE_AUTH_QUERY_KEYS) url.searchParams.delete(key);
  return `${url.pathname}${url.search}`;
}

function serializeError(err: unknown) {
  if (err instanceof Error) {
    return { name: err.name, message: err.message, stack: err.stack };
  }
  return err;
}

function emit(level: LogLevel, msg: string, fields: LogFields = {}) {
  const entry: Record<string, unknown> = {
    ts: new Date().toISOString(),
    level,
    msg,
    ...fields,
  };
  if ('error' in entry) entry.error = serializeError(entry.error);
  const line = JSON.stringify(entry);
  if (level === 'error') console.error(line);
  else if (level === 'warn') console.warn(line);
  else console.log(line);
}

export const log = {
  debug: (msg: string, fields?: LogFields) => emit('debug', msg, fields),
  info: (msg: string, fields?: LogFields) => emit('info', msg, fields),
  warn: (msg: string, fields?: LogFields) => emit('warn', msg, fields),
  error: (msg: string, fields?: LogFields) => emit('error', msg, fields),
};
