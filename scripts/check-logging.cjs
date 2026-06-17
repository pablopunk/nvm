#!/usr/bin/env node
const fs = require('node:fs');
const path = require('node:path');

// ── config ──────────────────────────────────────────────────────────────────

const ROOT = process.cwd();
const LOGGER_FILE = path.join(ROOT, 'src', 'electron', 'logger.ts');

// ── helpers ─────────────────────────────────────────────────────────────────

function fail(message) {
  console.error(`Logging check failed: ${message}`);
  process.exitCode = 1;
}

function relative(filePath) {
  return path.relative(ROOT, filePath);
}

// ── main ────────────────────────────────────────────────────────────────────

function main() {
  const source = fs.readFileSync(LOGGER_FILE, 'utf8');
  const rel = relative(LOGGER_FILE);

  // 1. Log file name must be 'nevermind.log' (not the old debug.log)
  if (
    source.includes("LOG_FILE_NAME = 'debug.log'") ||
    !source.includes("LOG_FILE_NAME = 'nevermind.log'")
  ) {
    fail(
      `${rel}: LOG_FILE_NAME must be 'nevermind.log'.\n` +
        `  Per src/docs/logging.md, the canonical log file is nevermind.log in Electron's logs directory.`,
    );
  }

  // 2. readRecentLogs must be bounded by MAX_RECENT_LIMIT
  if (
    !source.includes('MAX_RECENT_LIMIT') ||
    !source.includes('MAX_LOG_LINES')
  ) {
    fail(
      `${rel}: readRecentLogs must enforce MAX_RECENT_LIMIT and MAX_LOG_LINES bounds.\n` +
        `  Per src/docs/logging.md, logs must not include unbounded reads.`,
    );
  }

  // 3. Log writes go through a central write() that JSON-serializes via electron-log
  if (!source.includes('log[level](JSON.stringify(entry))')) {
    fail(
      `${rel}: log writes must go through JSON.stringify via electron-log.\n` +
        `  Per src/docs/logging.md, production logs must be structured and bounded.`,
    );
  }

  // 4. serializeData strips Error objects to safe fields (no raw stacks with secrets)
  if (!source.includes('data instanceof Error')) {
    fail(
      `${rel}: serializeData must strip Error objects to name/message/stack only.\n` +
        `  Per src/docs/logging.md, production logs must not include arbitrary secrets.`,
    );
  }

  if (process.exitCode) {
    console.error('\nLogging checks failed. See errors above.');
    process.exit(1);
  }

  console.log('Logging checks passed');
  console.log(`  Log file:     nevermind.log`);
  console.log(`  Read bounds:  enforced`);
  console.log(`  Write safety: JSON-structured`);
}

main();
