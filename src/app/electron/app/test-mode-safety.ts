// biome-ignore-all lint: Test-mode allowlists moved verbatim from main.ts.
import {
  isNvmDiagnosticTestMode,
  isNvmTestMode,
} from '../platform/test-mode-flags';
import { searchActionIsVisibleInTestMode } from '../search/search-snapshot';

export function testModeSafeAction() {
  return {
    id: 'test:confirm-safe-action',
    kind: 'test-action',
    title: 'Test: Confirm safe action',
    subtitle: 'In-memory deterministic Electron smoke action',
    icon: 'check',
    score: 100,
  };
}

export function testModeExtensionIsSafe(extensionId: string) {
  if (
    isNvmDiagnosticTestMode &&
    ['nevermind.diagnostics', 'qa.diagnostic-smoke'].includes(extensionId)
  )
    return true;
  return [
    'nevermind.system',
    'nevermind.extensions',
    'nevermind.permissions',
    'nevermind.floating-notes',
    'pab53.lifecycle',
    'pab53.legacy',
    'pab53.discovered',
    'pab85.search-progressive',
  ].includes(extensionId);
}

export function testModePaletteActionIsSafe(action) {
  return searchActionIsVisibleInTestMode(action, {
    isSafeExtension: testModeExtensionIsSafe,
    progressiveRootExtensionId: 'pab85.search-progressive',
    testMode: isNvmTestMode,
  });
}
