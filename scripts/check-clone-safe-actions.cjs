#!/usr/bin/env node
const {
  ELECTRON_SOURCE_SEPARATOR,
  readElectronSources,
} = require('./lib/read-electron-sources.cjs');

const sourceFiles = readElectronSources().split(ELECTRON_SOURCE_SEPARATOR);

function someSourceFileMatches(pattern) {
  return sourceFiles.some((source) => pattern.test(source));
}

function fail(message) {
  console.error(`Clone-safety check failed: ${message}`);
  process.exit(1);
}

if (
  !someSourceFileMatches(
    /function\s+normalizeViewItems[\s\S]*const\s+\{\s*run,\s*__handler,\s*action,\s*\.\.\.safeItem\s*\}\s*=\s*item/,
  )
) {
  fail(
    'normalizeViewItems must strip raw run/__handler/action fields before renderer IPC',
  );
}

if (
  !someSourceFileMatches(
    /function\s+normalizeActionPanel[\s\S]*const\s+\{\s*lazyActions,\s*\.\.\.safeSection\s*\}\s*=\s*section/,
  )
) {
  fail('normalizeActionPanel must strip lazyActions after normalizing them');
}

// Note: prepareRootActionForRenderer already strips handlers so sorted is clone-safe.
// The discarded structuredClone(sorted) was removed per #33. searchActions returns the
// snapshot builder's results, and the builder prepares every action for the renderer.
if (
  !someSourceFileMatches(
    /async\s+function\s+searchActions[\s\S]*return\s+createSearchSnapshotBuilder\(/,
  )
) {
  fail('searchActions must return the snapshot builder results for IPC');
}

if (
  !someSourceFileMatches(
    /function\s+createSearchSnapshotBuilder[\s\S]*prepare:\s*prepareRootActionForRenderer/,
  )
) {
  fail(
    'createSearchSnapshotBuilder must prepare actions for the renderer before IPC',
  );
}

if (
  !someSourceFileMatches(
    /async\s+function\s+executeActionForIpc[\s\S]*structuredClone\(result\)/,
  )
) {
  fail(
    'executeActionForIpc must structuredClone-check action results before returning through IPC',
  );
}

if (
  !someSourceFileMatches(
    /function\s+normalizeView[\s\S]*refresh:\s+registerViewRefreshForRenderer\(view\.refresh, entry, view\)/,
  )
) {
  fail(
    'normalizeView must turn refresh actions into opaque host-owned refresh handles',
  );
}

if (
  !someSourceFileMatches(
    /function\s+registerViewRefreshForRenderer[\s\S]*const\s+\{\s*action,\s*\.\.\.safeRefresh\s*\}\s*=\s*refresh[\s\S]*return\s+\{\s*\.\.\.safeRefresh,\s*id:\s*refreshId\s*\}/,
  )
) {
  fail(
    'registerViewRefreshForRenderer must strip executable refresh actions from renderer IPC payloads',
  );
}

if (
  !someSourceFileMatches(
    /ipcHandleMeasured\('view:refresh',[\s\S]*deps\.refreshViewForIpc/,
  )
) {
  fail(
    'view refresh must execute through the host-owned view:refresh IPC handler',
  );
}

console.log('Clone-safety checks passed');
