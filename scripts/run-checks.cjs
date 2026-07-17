'use strict';

// Run all check scripts sequentially, failing fast on first error.
const { spawnSync } = require('node:child_process');
const path = require('node:path');

const checks = [
  'check-pnpm-build-policy.cjs',
  'ensure-electron.test.cjs',
  'check-packaged-runtime-imports.cjs',
  'check-design-system.cjs',
  'check-internal-extensions.cjs',
  'check-clone-safe-actions.cjs',
  'check-packaged-resources.cjs',
  'check-backend-contract-fixtures.cjs',
  'check-extension-fixture-coverage.cjs',
  'check-extension-trust-copy.cjs',
  'check-os-platform-boundaries.cjs',
  'check-runtime-nonblocking.cjs',
  'check-window-config.cjs',
  'check-appearance-contract.cjs',
  'check-logging.cjs',
  'check-backend-api-major.cjs',
];

let failed = false;
for (const script of checks) {
  const scriptPath = path.join(__dirname, script);
  const result = spawnSync(process.execPath, [scriptPath], {
    stdio: 'inherit',
    timeout: 30_000,
  });
  if (result.status !== 0) {
    console.error(`\n  check failed: ${script}`);
    failed = true;
    break;
  }
}
process.exit(failed ? 1 : 0);
