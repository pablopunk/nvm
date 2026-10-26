const { spawnSync } = require('node:child_process');

const RELEVANT_FILES = new Set([
  '.npmrc',
  'electron-builder.yml',
  'electron.vite.config.ts',
  '.github/scripts/electron-smoke-paths.cjs',
  '.github/scripts/windows-package-paths.cjs',
  'mise.toml',
  'package.json',
  'pnpm-lock.yaml',
  'pnpm-workspace.yaml',
  'scripts/after-pack.cjs',
  'scripts/file-size-baseline.json',
  'scripts/check-packaged-resources.cjs',
  'scripts/verify-packaged-dependencies.cjs',
  'scripts/packaged-diagnostics-smoke.cjs',
  'src/app/electron/app/test-mode-ipc.ts',
  'src/app/electron/main.ts',
  'src/app/electron/platform/logger.ts',
  'src/app/electron/platform/test-mode-flags.ts',
  'src/app/electron/platform/test-mode.ts',
  'src/app/electron/preload.ts',
  'src/app/electron/diagnostics/backend-observability.ts',
  'src/app/palette/preload-api.ts',
  'src/app/shared/ai-context-window.ts',
  'src/backend/src/lib/limits.ts',
  'src/backend/src/lib/proxy.ts',
  'src/backend/src/pages/api/contract-routes.test.ts',
  '.github/workflows/ci.yml',
  '.github/workflows/pi-updates.yml',
]);

function isPiRelevantPath(filePath) {
  const normalized = filePath.replaceAll('\\', '/').replace(/^\.\//, '');
  if (RELEVANT_FILES.has(normalized)) return true;
  return (
    normalized.startsWith('src/app/electron/ai/') ||
    normalized.startsWith('src/app/electron/backend/') ||
    normalized.startsWith('src/app/electron/diagnostics/') ||
    normalized.startsWith('src/app/shared/ai-') ||
    normalized.startsWith(
      'src/app/resources/skills/nevermind-extension-builder/',
    ) ||
    normalized.startsWith('src/backend/src/lib/limits.') ||
    normalized.startsWith('src/backend/src/lib/proxy.') ||
    normalized.startsWith('src/backend/src/pages/api/contract-routes.') ||
    normalized.startsWith('.github/scripts/pi-') ||
    normalized.startsWith('.github/scripts/verify-pi-') ||
    normalized.startsWith('.github/scripts/verify-packaged-pi-') ||
    normalized.startsWith('.github/scripts/packaged-') ||
    normalized.startsWith('scripts/pi-') ||
    normalized.startsWith('scripts/packaged-pi-') ||
    normalized.startsWith('scripts/check-packaged-resources.') ||
    normalized.startsWith('scripts/verify-packaged-dependencies.') ||
    normalized.startsWith('scripts/packaged-diagnostics-smoke.')
  );
}

function classifyPiChanges(filePaths) {
  return filePaths.some(isPiRelevantPath);
}

function changedPaths(baseSha, headSha) {
  if (!baseSha || !headSha)
    throw new Error(
      'Both base and head commits are required for classification.',
    );
  for (const sha of [baseSha, headSha]) {
    const check = spawnSync('git', ['cat-file', '-e', `${sha}^{commit}`], {
      encoding: 'utf8',
    });
    if (check.status !== 0)
      throw new Error(
        `Cannot resolve ${sha} as a commit for Pi classification.`,
      );
  }
  const diff = spawnSync(
    'git',
    ['diff', '--name-status', '-z', '--find-renames', baseSha, headSha],
    { encoding: 'buffer' },
  );
  if (diff.status !== 0)
    throw new Error(
      `Could not establish Pi classification diff: ${diff.stderr}`,
    );
  return parseNameStatus(diff.stdout.toString('utf8'));
}

function parseNameStatus(output) {
  const fields = output.split('\0').filter(Boolean);
  const paths = [];
  for (let index = 0; index < fields.length; index += 1) {
    const status = fields[index];
    if (/^[RC]/.test(status)) {
      paths.push(fields[index + 1], fields[index + 2]);
      index += 2;
    } else {
      paths.push(fields[index + 1]);
      index += 1;
    }
  }
  return paths.filter(Boolean);
}

if (require.main === module) {
  try {
    const paths = changedPaths(process.argv[2], process.argv[3]);
    console.log(`relevant=${classifyPiChanges(paths)}`);
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}

module.exports = {
  classifyPiChanges,
  changedPaths,
  isPiRelevantPath,
  parseNameStatus,
};
