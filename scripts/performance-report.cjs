#!/usr/bin/env node
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const names = ['Nevermind', 'nvm'];
const candidates =
  process.platform === 'darwin'
    ? names.map((name) =>
        path.join(os.homedir(), 'Library', 'Logs', name, 'nevermind.log'),
      )
    : process.platform === 'win32'
      ? names.map((name) =>
          path.join(
            process.env.APPDATA ||
              path.join(os.homedir(), 'AppData', 'Roaming'),
            name,
            'logs',
            'nevermind.log',
          ),
        )
    : names.map((name) =>
        path.join(
          process.env.XDG_CONFIG_HOME || path.join(os.homedir(), '.config'),
          name,
          'logs',
          'nevermind.log',
        ),
      );
const logPath =
  process.argv[2] || candidates.find((candidate) => fs.existsSync(candidate));
if (!logPath || !fs.existsSync(logPath)) {
  console.error(
    'Performance log not found. Pass a log path as the first argument.',
  );
  process.exit(1);
}

const entries = fs
  .readFileSync(logPath, 'utf8')
  .trim()
  .split('\n')
  .map((line) => {
    try {
      return JSON.parse(line.slice(line.indexOf('{')));
    } catch {
      return null;
    }
  })
  .filter((entry) => entry?.message === 'performance.trace');

const groups = new Map();
for (const entry of entries) {
  const data = entry.data || {};
  if (typeof data.operation !== 'string' || typeof data.durationMs !== 'number')
    continue;
  const key = data.operation;
  const values = groups.get(key) || [];
  values.push(data.durationMs);
  groups.set(key, values);
}

function percentile(values, fraction) {
  const sorted = [...values].sort((a, b) => a - b);
  return sorted[
    Math.min(sorted.length - 1, Math.floor((sorted.length - 1) * fraction))
  ];
}

const report = [...groups]
  .map(([operation, values]) => ({
    operation,
    count: values.length,
    p50: Number(percentile(values, 0.5).toFixed(2)),
    p95: Number(percentile(values, 0.95).toFixed(2)),
    max: Number(Math.max(...values).toFixed(2)),
  }))
  .sort((left, right) => right.p95 - left.p95);

console.log(
  JSON.stringify(
    { logPath, traceCount: entries.length, operations: report },
    null,
    2,
  ),
);
