const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');

const args = process.argv.slice(2);
let sourceArg = null;
let dryRun = false;
let force = false;
let includeExamples = false;

for (let i = 0; i < args.length; i += 1) {
  const arg = args[i];
  if (arg === '--source') {
    sourceArg = args[i + 1];
    i += 1;
  } else if (arg === '--dry-run') {
    dryRun = true;
  } else if (arg === '--force') {
    force = true;
  } else if (arg === '--include-examples') {
    includeExamples = true;
  } else if (arg === '--help' || arg === '-h') {
    printHelp();
    process.exit(0);
  } else {
    fail(`Unknown argument: ${arg}`);
  }
}

const currentRoot = git(['rev-parse', '--show-toplevel']).trim();
const sourceRoot = path.resolve(sourceArg ?? canonicalCheckout(currentRoot));

if (samePath(currentRoot, sourceRoot))
  fail('Current worktree is already the canonical checkout.');
if (!fs.existsSync(sourceRoot))
  fail(`Source checkout does not exist: ${sourceRoot}`);

const files = findEnvFiles(sourceRoot);
if (files.length === 0) {
  console.log(`No env files found in ${sourceRoot}`);
  process.exit(0);
}

let copied = 0;
let skipped = 0;
const conflicts = [];

for (const rel of files) {
  const source = path.join(sourceRoot, rel);
  const target = path.join(currentRoot, rel);

  if (fs.existsSync(target)) {
    const same = fs.readFileSync(source).equals(fs.readFileSync(target));
    if (same) {
      skipped += 1;
      console.log(`same    ${rel}`);
      continue;
    }
    if (!force) {
      conflicts.push(rel);
      console.log(`exists  ${rel}`);
      continue;
    }
  }

  copied += 1;
  console.log(`${dryRun ? 'would copy' : 'copy   '} ${rel}`);
  if (!dryRun) {
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.copyFileSync(source, target);
    fs.chmodSync(target, fs.statSync(source).mode & 0o777);
  }
}

if (conflicts.length > 0) {
  console.error(
    `\nRefusing to overwrite ${conflicts.length} existing env file(s). Re-run with --force to replace them.`,
  );
  process.exit(1);
}

console.log(
  `\nDone: ${dryRun ? 'would copy' : 'copied'} ${copied}, unchanged ${skipped}.`,
);

function canonicalCheckout(current) {
  const blocks = git(['worktree', 'list', '--porcelain'])
    .trim()
    .split(/\n\n+/)
    .filter(Boolean);
  const worktrees = blocks
    .map((block) => {
      const lines = block.split('\n');
      return {
        path: lines
          .find((line) => line.startsWith('worktree '))
          ?.slice('worktree '.length),
        branch: lines
          .find((line) => line.startsWith('branch '))
          ?.slice('branch '.length),
      };
    })
    .filter((entry) => entry.path);

  const main = worktrees.find(
    (entry) =>
      entry.branch === 'refs/heads/main' && !samePath(entry.path, current),
  );
  if (main) return main.path;

  const other = worktrees.find((entry) => !samePath(entry.path, current));
  if (other) return other.path;

  fail(
    'Could not find another checkout. Pass --source /path/to/canonical/checkout.',
  );
}

function findEnvFiles(root) {
  const found = [];
  walk(root, '');
  return found.sort();

  function walk(absDir, relDir) {
    for (const dirent of fs.readdirSync(absDir, { withFileTypes: true })) {
      const rel = path.join(relDir, dirent.name);
      const abs = path.join(absDir, dirent.name);
      if (dirent.isDirectory()) {
        if (['.git', 'node_modules', 'dist', 'release'].includes(dirent.name))
          continue;
        walk(abs, rel);
        continue;
      }
      if (!dirent.isFile()) continue;
      if (!isEnvFile(dirent.name)) continue;
      found.push(rel);
    }
  }
}

function isEnvFile(name) {
  if (name === '.env.example' && !includeExamples) return false;
  return name === '.env' || name.startsWith('.env.');
}

function git(command) {
  try {
    return execFileSync('git', command, { encoding: 'utf8' });
  } catch (error) {
    fail(error.message);
  }
}

function samePath(a, b) {
  return path.resolve(a) === path.resolve(b);
}

function fail(message) {
  console.error(message);
  process.exit(1);
}

function printHelp() {
  console.log(
    `Copy env files from the canonical checkout into this worktree.\n\nUsage:\n  pnpm worktree:env:copy [--force] [--dry-run] [--source PATH] [--include-examples]\n\nBy default, the source is the main-branch checkout from git worktree list. Existing\ndifferent env files are left untouched unless --force is passed. Secrets are never\nprinted; only relative file paths are shown.`,
  );
}
