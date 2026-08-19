export const isNvmTestMode = process.env.NVM_TEST_MODE === '1';

/** Keep local Electron smoke windows off the user's desktop unless CI opts in. */
export const isNvmHeadlessTestMode =
  isNvmTestMode && process.env.NVM_TEST_HEADLESS !== '0';
