const assert = require('node:assert/strict');
const test = require('node:test');
const {
  classifyPiChanges,
  parseNameStatus,
} = require('./pi-change-classification.cjs');

test('Pi classifier includes SDK, policies, integration, backend, and packaging paths', function relevantPaths() {
  for (const filePath of [
    'package.json',
    'pnpm-lock.yaml',
    'pnpm-workspace.yaml',
    '.npmrc',
    'mise.toml',
    'src/app/electron/ai/ai.ts',
    'src/app/electron/ai/pi-session-integration.test.ts',
    'src/app/electron/backend/nevermind-auth.ts',
    'src/app/electron/app/test-mode-ipc.ts',
    'src/app/electron/diagnostics/backend-observability.ts',
    'src/app/electron/platform/test-mode-flags.ts',
    'src/app/electron/platform/test-mode.ts',
    'src/app/electron/preload.ts',
    'src/app/palette/preload-api.ts',
    'src/app/electron/platform/logger.ts',
    'src/app/shared/ai-context-window.ts',
    'src/app/shared/ai-chat-context.ts',
    'src/app/resources/skills/nevermind-extension-builder/SKILL.md',
    'src/backend/src/lib/proxy.ts',
    'src/backend/src/lib/limits.ts',
    'src/backend/src/pages/api/contract-routes.test.ts',
    '.github/workflows/pi-updates.yml',
    '.github/scripts/pi-change-classification.cjs',
    '.github/scripts/verify-pi-integration-evidence.cjs',
    '.github/scripts/verify-packaged-pi-electron-evidence.cjs',
    '.github/scripts/electron-smoke-paths.cjs',
    '.github/scripts/windows-package-paths.cjs',
    'scripts/check-packaged-resources.cjs',
    'scripts/packaged-diagnostics-smoke.cjs',
    'scripts/file-size-baseline.json',
    'electron-builder.yml',
    'electron.vite.config.ts',
  ])
    assert.equal(classifyPiChanges([filePath]), true, filePath);
});

test('Pi classifier excludes unrelated edits and handles deleted or renamed files by path', function unrelatedAndHistoricalPaths() {
  assert.equal(
    classifyPiChanges(['src/app/palette/components/command-row.tsx']),
    false,
  );
  assert.equal(classifyPiChanges(['docs/solutions/unrelated.md']), false);
  assert.equal(classifyPiChanges(['src/app/shared/public-origin.ts']), false);
  assert.equal(
    classifyPiChanges(['src/app/electron/ai/removed.test.ts']),
    true,
  );
  assert.equal(classifyPiChanges(['src/app/electron/ai/renamed.ts']), true);
  assert.deepEqual(
    parseNameStatus(
      'D\0src/app/electron/ai/removed.ts\0R100\0src/app/electron/ai/old.ts\0src/app/palette/new.ts\0',
    ),
    [
      'src/app/electron/ai/removed.ts',
      'src/app/electron/ai/old.ts',
      'src/app/palette/new.ts',
    ],
  );
  assert.equal(
    classifyPiChanges(
      parseNameStatus(
        'R100\0src/app/electron/ai/old.ts\0src/app/palette/new.ts\0',
      ),
    ),
    true,
  );
});
