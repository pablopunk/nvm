import { app } from 'electron';

export const NEVERMIND_DESKTOP_API_VERSION = '1';

export function nevermindDesktopHeaders(headers: Record<string, string> = {}) {
  return {
    ...headers,
    'X-Nevermind-Client': 'desktop',
    'X-Nevermind-Client-Version': app.getVersion(),
    'X-Nevermind-API-Version': NEVERMIND_DESKTOP_API_VERSION,
    'X-Nevermind-Platform': process.platform,
    'X-Nevermind-Arch': process.arch,
  };
}
