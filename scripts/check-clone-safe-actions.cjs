#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');

const source = [
  fs.readFileSync(
    path.join(process.cwd(), 'src', 'electron', 'main.ts'),
    'utf8',
  ),
  fs.readFileSync(
    path.join(process.cwd(), 'src', 'electron', 'app-ipc-handlers.ts'),
    'utf8',
  ),
].join('\n');

function fail(message) {
  // biome-ignore lint/suspicious/noConsole: CLI script
  console.error(`Clone-safety check failed: ${message}`);
  process.exit(1);
}

if (
  !/function\s+normalizeViewItems[\s\S]*const\s+\{\s*run,\s*__handler,\s*action,\s*\.\.\.safeItem\s*\}\s*=\s*item/.test(
    source,
  )
) {
  fail(
    'normalizeViewItems must strip raw run/__handler/action fields before renderer IPC',
  );
}

if (
  !/function\s+normalizeActionPanel[\s\S]*const\s+\{\s*lazyActions,\s*\.\.\.safeSection\s*\}\s*=\s*section/.test(
    source,
  )
) {
  fail('normalizeActionPanel must strip lazyActions after normalizing them');
}

// Note: prepareRootActionForRenderer already strips handlers so sorted is clone-safe.
// The discarded structuredClone(sorted) was removed per #33. Verify the function returns sorted.
if (!/async\s+function\s+searchActions[\s\S]*return\s+sorted/.test(source)) {
  fail('searchActions must return the sorted results for IPC');
}

if (
  !/async\s+function\s+executeActionForIpc[\s\S]*structuredClone\(result\)/.test(
    source,
  )
) {
  fail(
    'executeActionForIpc must structuredClone-check action results before returning through IPC',
  );
}

if (
  !/function\s+normalizeView[\s\S]*refresh:\s+registerViewRefreshForRenderer\(view\.refresh, entry, view\)/.test(
    source,
  )
) {
  fail(
    'normalizeView must turn refresh actions into opaque host-owned refresh handles',
  );
}

if (
  !/function\s+registerViewRefreshForRenderer[\s\S]*const\s+\{\s*action,\s*\.\.\.safeRefresh\s*\}\s*=\s*refresh[\s\S]*return\s+\{\s*\.\.\.safeRefresh,\s*id:\s*refreshId\s*\}/.test(
    source,
  )
) {
  fail(
    'registerViewRefreshForRenderer must strip executable refresh actions from renderer IPC payloads',
  );
}

if (!/ipcMain\.handle\('view:refresh',[\s\S]*refreshViewForIpc/.test(source)) {
  fail(
    'view refresh must execute through the host-owned view:refresh IPC handler',
  );
}

// biome-ignore lint/suspicious/noConsole: CLI script
console.log('Clone-safety checks passed');
