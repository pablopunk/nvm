import { Axiom } from '@axiomhq/js';

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

const axiomToken = process.env.AXIOM_TOKEN;
const axiomDataset = process.env.AXIOM_DATASET;
const axiomEdge = process.env.AXIOM_EDGE || undefined;

const axiom = axiomToken && axiomDataset
  ? new Axiom({
      token: axiomToken,
      edge: axiomEdge,
      onError: (error) => console.error('[axiom] failed to ingest logs', error),
    })
  : undefined;

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

  if (axiom && axiomDataset) axiom.ingest(axiomDataset, entry);
}

/**
 * Flushes the Axiom client's in-memory batch. Call this from a request-lifetime
 * hook so serverless invocations don't end before queued log events are sent.
 */
export async function flushLogs() {
  try {
    await axiom?.flush();
  } catch (error) {
    console.error('[axiom] failed to flush logs', error);
  }
}

export const log = {
  debug: (msg: string, fields?: LogFields) => emit('debug', msg, fields),
  info: (msg: string, fields?: LogFields) => emit('info', msg, fields),
  warn: (msg: string, fields?: LogFields) => emit('warn', msg, fields),
  error: (msg: string, fields?: LogFields) => emit('error', msg, fields),
};
