// biome-ignore-all lint: Moved verbatim from main.ts; keeps its established host-side conventions.
import type { NevermindDeviceSignInStatus } from '../../shared/nevermind-auth';
import {
  getNevermindAuth,
  onNevermindDeviceSignInChanged,
  setActiveNevermindAuthBaseUrl,
  signInToNevermind,
} from './nevermind-auth';
import { switchNevermindBackendEnvironment as switchBackendEnvironment } from './nevermind-backend-environment';
import {
  checkNevermindCompatibility,
  invalidateNevermindCompatibilityCache,
  warmNevermindCompatibilityCache,
} from './nevermind-compatibility';
import { resolvesToUnsafeNevermindAddress } from './nevermind-url';

export type NevermindSignInDeps = {
  selectedNevermindEnvironment: () => any;
  setSelectedEnvironment: (selection: any) => void;
  setActiveBaseUrl: (baseUrl: string | null) => void;
  isPackaged: () => boolean;
  scheduleSaveState: () => void;
  invalidateExtensionRootItems: () => void;
  sendToPalette: (channel: string, payload: unknown) => void;
  beginAiChatSendSuspension: () => void;
  endAiChatSendSuspension: () => void;
  disposeAiSessions: () => Promise<unknown> | undefined;
  broadcastAuthChanged: (status: {
    authed: boolean;
    userId?: string;
    email?: string;
    role?: string;
  }) => void;
};

export function createNevermindSignIn(deps: NevermindSignInDeps) {
  const {
    selectedNevermindEnvironment,
    setSelectedEnvironment,
    setActiveBaseUrl,
    isPackaged,
    scheduleSaveState,
    invalidateExtensionRootItems,
    sendToPalette,
    beginAiChatSendSuspension,
    endAiChatSendSuspension,
    disposeAiSessions,
    broadcastAuthChanged,
  } = deps;

  async function signInToSelectedNevermindEnvironment(
    onStatusChange?: (status: NevermindDeviceSignInStatus | null) => void,
  ) {
    const selected = selectedNevermindEnvironment();
    const unsubscribe = onNevermindDeviceSignInChanged((status) => {
      invalidateExtensionRootItems();
      sendToPalette('nevermind:device-sign-in-changed', status);
      onStatusChange?.(status);
    });
    try {
      const result = await signInToNevermind({
        baseUrl: selected.baseUrl,
        environment: selected.environment,
      });
      if (result.ok) {
        setActiveBaseUrl(result.auth.baseUrl);
        setActiveNevermindAuthBaseUrl(result.auth.baseUrl);
        warmNevermindCompatibilityCache(result.auth.baseUrl);
      }
      return result;
    } finally {
      unsubscribe();
    }
  }

  async function switchNevermindBackendEnvironment(input: {
    environment: 'development' | 'production' | 'pr_preview' | 'custom';
    baseUrl?: string;
  }) {
    beginAiChatSendSuspension();
    try {
      return await switchBackendEnvironment(input, {
        isPackaged: isPackaged(),
        selectedEnvironment: selectedNevermindEnvironment,
        resolvesToUnsafeAddress: resolvesToUnsafeNevermindAddress,
        invalidateCompatibilityCache: invalidateNevermindCompatibilityCache,
        checkCompatibility: checkNevermindCompatibility,
        setSelectedEnvironment,
        scheduleSaveState,
        setActiveAuthBaseUrl: setActiveNevermindAuthBaseUrl,
        getAuth: getNevermindAuth,
        signIn: signInToSelectedNevermindEnvironment,
        setActiveBaseUrl,
        warmCompatibilityCache: warmNevermindCompatibilityCache,
        disposeAiSessions,
        invalidateExtensionRootItems,
        broadcastAuthChanged,
      });
    } finally {
      endAiChatSendSuspension();
    }
  }

  return {
    signInToSelectedNevermindEnvironment,
    switchNevermindBackendEnvironment,
  };
}
