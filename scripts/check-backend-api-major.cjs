#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');

const apiV2Dir = path.join(process.cwd(), 'backend/src/pages/api/v2');
if (!fs.existsSync(apiV2Dir)) {
  // biome-ignore lint/suspicious/noConsole: CLI script
  console.log('Backend API major check passed (no /api/v2 routes present)');
  process.exit(0);
}

const migrationDoc = path.join(process.cwd(), 'src/docs/backend-api-v2.md');
if (!fs.existsSync(migrationDoc)) {
  // biome-ignore lint/suspicious/noConsole: CLI script
  console.error(
    '/api/v2 routes require src/docs/backend-api-v2.md with breaking-change and sunset details.',
  );
  process.exit(1);
}

const text = fs.readFileSync(migrationDoc, 'utf8').toLowerCase();
for (const required of [
  'breaking change',
  'sunset',
  'client count',
  'update message',
]) {
  if (!text.includes(required)) {
    // biome-ignore lint/suspicious/noConsole: CLI script
    console.error(`src/docs/backend-api-v2.md must mention: ${required}`);
    process.exit(1);
  }
}

// biome-ignore lint/suspicious/noConsole: CLI script
console.log('Backend API major check passed (/api/v2 documented)');
