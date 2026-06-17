#!/usr/bin/env node
const fs = require('node:fs');
const path = require('node:path');

const root = path.join(process.cwd(), 'backend/src/fixtures/contracts');

function walk(dir) {
  if (!fs.existsSync(dir)) return [];
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name);
    return entry.isDirectory() ? walk(full) : [full];
  });
}

const files = walk(root).filter((file) => file.endsWith('.json'));
if (files.length === 0) {
  console.error(
    'No backend contract fixtures found under backend/src/fixtures/contracts',
  );
  process.exit(1);
}

for (const file of files) {
  let parsed;
  try {
    parsed = JSON.parse(fs.readFileSync(file, 'utf8'));
  } catch (error) {
    console.error(
      `Invalid JSON fixture: ${path.relative(process.cwd(), file)}`,
    );
    console.error(error);
    process.exit(1);
  }

  const relative = path.relative(process.cwd(), file);
  if (relative.endsWith('compatibility-manifest.json')) {
    for (const key of [
      'backend',
      'api',
      'desktop',
      'client',
      'features',
      'notices',
    ]) {
      if (!(key in parsed)) {
        console.error(
          `Compatibility manifest fixture missing ${key}: ${relative}`,
        );
        process.exit(1);
      }
    }
  }

  if (relative.endsWith('-error.json') && !parsed.error?.type) {
    console.error(`Error fixture missing error.type: ${relative}`);
    process.exit(1);
  }
}

console.log(
  `Backend contract fixture checks passed (${files.length} fixtures)`,
);
