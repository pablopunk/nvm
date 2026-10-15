#!/usr/bin/env node
const fs = require('node:fs');
const path = require('node:path');

// ── config ──────────────────────────────────────────────────────────────────

const ROOT = process.cwd();
const APP_DIR = path.join(ROOT, 'src', 'app');
const BASELINE_PATH = path.join(ROOT, 'scripts', 'file-size-baseline.json');
const MAX_LINES = 500;

// ── file discovery ──────────────────────────────────────────────────────────

function walkFiles(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const entryPath = path.join(directory, entry.name);
    return entry.isDirectory() ? walkFiles(entryPath) : [entryPath];
  });
}

function isSizeCheckedFile(file) {
  return (
    /\.tsx?$/.test(file) &&
    !/\.test\.tsx?$/.test(file) &&
    !/\.d\.ts$/.test(file)
  );
}

function countLines(file) {
  const text = fs.readFileSync(file, 'utf8');
  if (text === '') return 0;
  const lines = text.split('\n').length;
  return text.endsWith('\n') ? lines - 1 : lines;
}

function baselineKeyOf(file) {
  const [topDirectory] = path.relative(APP_DIR, file).split(path.sep);
  return `${topDirectory}/${path.basename(file)}`;
}

function relative(file) {
  return path.relative(ROOT, file);
}

function groupFilesByKey(files) {
  const groups = new Map();
  for (const file of files) {
    const key = baselineKeyOf(file);
    groups.set(key, [...(groups.get(key) ?? []), file]);
  }
  return groups;
}

function measureOversizedKeys(files) {
  const lineCounts = new Map(files.map((file) => [file, countLines(file)]));
  const failures = [];
  const oversized = new Map();
  for (const [key, group] of groupFilesByKey(files)) {
    if (!group.some((file) => lineCounts.get(file) > MAX_LINES)) continue;
    if (group.length > 1) {
      failures.push(
        `baseline key "${key}" is ambiguous: ${group.map(relative).join(', ')}. Rename one of the files so each oversized file has a unique <top directory>/<basename> key.`,
      );
      continue;
    }
    oversized.set(key, lineCounts.get(group[0]));
  }
  return { oversized, failures };
}

// ── baseline comparison ─────────────────────────────────────────────────────

function readBaseline() {
  if (!fs.existsSync(BASELINE_PATH)) {
    console.error(
      `File size check failed: ${relative(BASELINE_PATH)} is missing.`,
    );
    process.exit(1);
  }
  return JSON.parse(fs.readFileSync(BASELINE_PATH, 'utf8'));
}

function compareWithBaseline(oversized, baseline) {
  const failures = [];
  const hints = [];
  for (const [key, lines] of oversized) {
    const allowed = baseline[key];
    if (allowed === undefined) {
      failures.push(
        `${key} has ${lines} lines (limit ${MAX_LINES}) and is not in the baseline. Split the file instead of adding it to the baseline.`,
      );
    } else if (lines > allowed) {
      failures.push(
        `${key} grew to ${lines} lines (baseline ${allowed}). Move code out of the file instead of growing it.`,
      );
    } else if (lines < allowed) {
      hints.push(
        `${key} shrank to ${lines} lines (baseline ${allowed}); lower its entry in ${relative(BASELINE_PATH)} to ${lines}.`,
      );
    }
  }
  for (const key of Object.keys(baseline)) {
    if (!oversized.has(key)) {
      hints.push(
        `${key} is no longer over ${MAX_LINES} lines; remove its entry from ${relative(BASELINE_PATH)}.`,
      );
    }
  }
  return { failures, hints };
}

// ── main ────────────────────────────────────────────────────────────────────

function main() {
  const files = walkFiles(APP_DIR).filter(isSizeCheckedFile).sort();
  const measured = measureOversizedKeys(files);
  const compared = compareWithBaseline(measured.oversized, readBaseline());
  const failures = [...measured.failures, ...compared.failures];

  for (const hint of compared.hints) console.log(`hint: ${hint}`);
  if (failures.length > 0) {
    console.error(`File size check failed (${failures.length}):`);
    for (const failure of failures) console.error(`  ${failure}`);
    process.exit(1);
  }
  console.log(
    `File size checks passed (${measured.oversized.size} baselined files over ${MAX_LINES} lines)`,
  );
}

main();
