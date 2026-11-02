const { recordSigningSinceAfterPack } = require('./lib/packaging-timings.cjs');

// Wired only into signed macOS release packaging (see `dist:mac:arm64`):
// unsigned builds never reach afterSign and electron-builder warns when a user
// afterSign hook is configured without signing.
module.exports = async function recordMacSigningDuration(context) {
  recordSigningSinceAfterPack(
    `${context.electronPlatformName} codesign + notarization`,
  );
};
