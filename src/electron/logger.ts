import fs from 'node:fs/promises';
import path from 'node:path';
import { app } from 'electron';
import log from 'electron-log/main';

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';
export type LogSource = 'main' | 'renderer' | 'extension' | 'host';

export type LogEntry = {
  timestamp: string;
  level: LogLevel;
  source: LogSource;
  scope?: string;
  extensionId?: string;
  commandId?: string;
  message: string;
  data?: unknown;
};

export type RecentLogOptions = {
  limit?: number;
  level?: LogLevel;
  source?: LogSource;
  sinceMs?: number;
  query?: string;
  extensionId?: string;
};

const LOG_FILE_NAME = 'nevermind.log';
const MAX_LOG_LINES = 5_000;
const DEFAULT_RECENT_LIMIT = 200;
const MAX_RECENT_LIMIT = 1_000;

let configured = false;
let prettyConsole = false;

export function configureLogger(isDev: boolean) {
  if (configured) return;
  configured = true;
  prettyConsole = isDev;
  log.initialize();
  log.transports.file.level = 'debug';
  log.transports.file.fileName = LOG_FILE_NAME;
  log.transports.file.maxSize = 5 * 1024 * 1024;
  log.transports.file.format = '{text}';
  log.transports.file.resolvePathFn = () => logPath();
  log.transports.console.level = false;
  info('logger.ready', { path: logPath() });
}

export function logPath() {
  return path.join(app.getPath('logs'), LOG_FILE_NAME);
}

export function debug(
  message: string,
  data?: unknown,
  meta?: Partial<LogEntry>,
) {
  write('debug', message, data, meta);
}

export function info(
  message: string,
  data?: unknown,
  meta?: Partial<LogEntry>,
) {
  write('info', message, data, meta);
}

export function warn(
  message: string,
  data?: unknown,
  meta?: Partial<LogEntry>,
) {
  write('warn', message, data, meta);
}

export function error(
  message: string,
  data?: unknown,
  meta?: Partial<LogEntry>,
) {
  write('error', message, data, meta);
}

export function extensionLogger(extensionId: string, commandId?: string) {
  const meta = { source: 'extension' as const, extensionId, commandId };
  return {
    debug: (message: string, data?: unknown) => debug(message, data, meta),
    info: (message: string, data?: unknown) => info(message, data, meta),
    warn: (message: string, data?: unknown) => warn(message, data, meta),
    error: (message: string, data?: unknown) => error(message, data, meta),
    recent: (options: RecentLogOptions = {}) => readRecentLogs(options),
  };
}

export async function readRecentLogs(options: RecentLogOptions = {}) {
  const limit = Math.min(
    Math.max(Number(options.limit || DEFAULT_RECENT_LIMIT), 1),
    MAX_RECENT_LIMIT,
  );
  const text = await fs.readFile(logPath(), 'utf8').catch(() => '');
  if (!text) return [];
  const since = options.sinceMs ? Date.now() - Number(options.sinceMs) : 0;
  const query = options.query ? String(options.query).toLowerCase() : '';
  const entries = text
    .trimEnd()
    .split('\n')
    .slice(-MAX_LOG_LINES)
    .map(parseLine)
    .filter((entry): entry is LogEntry => Boolean(entry))
    .filter((entry) => !options.level || entry.level === options.level)
    .filter((entry) => !options.source || entry.source === options.source)
    .filter(
      (entry) =>
        !options.extensionId ||
        entry.extensionId === options.extensionId ||
        lineContains(entry, options.extensionId),
    )
    .filter((entry) => !since || Date.parse(entry.timestamp) >= since)
    .filter((entry) => !query || lineContains(entry, query));
  return entries.slice(-limit);
}

function write(
  level: LogLevel,
  message: string,
  data?: unknown,
  meta: Partial<LogEntry> = {},
) {
  const entry: LogEntry = {
    timestamp: new Date().toISOString(),
    level,
    source: meta.source || 'main',
    scope: meta.scope,
    extensionId: meta.extensionId,
    commandId: meta.commandId,
    message,
    data: serializeData(data),
  };
  log[level](JSON.stringify(entry));
  if (prettyConsole) writePrettyConsole(entry);
}

function writePrettyConsole(entry: LogEntry) {
  const time = entry.timestamp.slice(11, 23);
  const scope = [entry.source, entry.scope, entry.extensionId, entry.commandId]
    .filter(Boolean)
    .join(':');
  const prefix = `${time} ${entry.level.toUpperCase().padEnd(5)} ${scope || 'main'}`;
  const line = `${prefix} ${entry.message}`;
  const writer = entry.level === 'error' ? process.stderr : process.stdout;
  writer.write(`${line}\n`);
  if (entry.data !== undefined) writer.write(`${prettyData(entry.data)}\n`);
}

function prettyData(data: unknown) {
  if (typeof data === 'string') return `  ${data}`;
  return JSON.stringify(data, null, 2)
    .split('\n')
    .map((line) => `  ${line}`)
    .join('\n');
}

function parseLine(line: string) {
  const jsonStart = line.indexOf('{');
  if (jsonStart < 0) return null;
  try {
    const value = JSON.parse(line.slice(jsonStart));
    if (
      !value ||
      typeof value !== 'object' ||
      !value.timestamp ||
      !value.level ||
      !value.message
    )
      return null;
    return value as LogEntry;
  } catch {
    return null;
  }
}

function serializeData(data: unknown) {
  if (data == null) return undefined;
  if (data instanceof Error)
    return { name: data.name, message: data.message, stack: data.stack };
  try {
    return JSON.parse(
      JSON.stringify(data, (_key, value) => {
        if (value instanceof Error)
          return {
            name: value.name,
            message: value.message,
            stack: value.stack,
          };
        return value;
      }),
    );
  } catch {
    return String(data);
  }
}

function lineContains(entry: LogEntry, query: string) {
  return JSON.stringify(entry)
    .toLowerCase()
    .includes(String(query).toLowerCase());
}
