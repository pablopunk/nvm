declare const __NVM_TEST_PI_PACKAGED_BUILD__: boolean;

export const isNvmTestMode = process.env.NVM_TEST_MODE === '1';
const isNvmPackagedPiTestBuild =
  typeof __NVM_TEST_PI_PACKAGED_BUILD__ === 'boolean' &&
  __NVM_TEST_PI_PACKAGED_BUILD__;
export const isNvmPackagedPiTestMode =
  isNvmPackagedPiTestBuild &&
  isNvmTestMode &&
  process.env.NVM_TEST_PI_PACKAGED === '1';
export const isNvmDiagnosticTestMode =
  isNvmTestMode && process.env.NVM_TEST_DIAGNOSTICS === '1';

/** Keep local Electron smoke windows off the user's desktop unless CI opts in. */
export const isNvmHeadlessTestMode =
  isNvmTestMode && process.env.NVM_TEST_HEADLESS !== '0';
