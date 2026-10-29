// biome-ignore-all lint: Moved verbatim from main.ts; keeps its established host-side conventions.
import type { NevermindDeviceSignInStatus } from '../../shared/nevermind-auth';

export type NevermindAuthBroadcastDeps = {
  sendToPalette: (channel: string, payload: unknown) => void;
  broadcastToExtensionWindows: (channel: string, payload: unknown) => void;
  isPaletteSender: (sender: unknown) => boolean;
  getExtensionWindowStateForSender: (sender: unknown) => unknown;
};

export function createNevermindAuthBroadcast(deps: NevermindAuthBroadcastDeps) {
  const {
    sendToPalette,
    broadcastToExtensionWindows,
    isPaletteSender,
    getExtensionWindowStateForSender,
  } = deps;

  function broadcastAuthChanged(status: {
    authed: boolean;
    userId?: string;
    email?: string;
    role?: string;
  }) {
    sendToPalette('nevermind:auth-changed', status);
    broadcastToExtensionWindows('nevermind:auth-changed', status);
  }

  function sendNevermindDeviceSignInStatus(
    sender: unknown,
    status: NevermindDeviceSignInStatus | null,
  ) {
    if (isPaletteSender(sender)) return;
    if (!getExtensionWindowStateForSender(sender)) return;
    (sender as { send(channel: string, ...args: unknown[]): void }).send(
      'nevermind:device-sign-in-changed',
      status,
    );
  }

  return { broadcastAuthChanged, sendNevermindDeviceSignInStatus };
}
