import type {
  NevermindAuthSnapshot,
  NevermindEnvironment,
} from './nevermind-auth';
import type { NevermindCompatibilityManifest } from './nevermind-compatibility';
import { parsePublicOrigin } from '../shared/public-origin';

export const PRODUCTION_NEVERMIND_BASE_URL = 'https://api.nvm.fyi';

export type NevermindBackendEnvironment = {
  environment: NevermindEnvironment;
  baseUrl: string;
};

type SignInResult =
  | { ok: true; auth: NonNullable<NevermindAuthSnapshot> }
  | { ok: false; error: string };

export type NevermindBackendEnvironmentDeps = {
  isPackaged: boolean;
  selectedEnvironment: () => NevermindBackendEnvironment;
  resolvesToUnsafeAddress: (hostname: string) => Promise<boolean>;
  invalidateCompatibilityCache: (baseUrl: string) => Promise<void>;
  checkCompatibility: (
    baseUrl: string,
  ) => Promise<NevermindCompatibilityManifest | null>;
  setSelectedEnvironment: (selection: NevermindBackendEnvironment) => void;
  scheduleSaveState: () => void;
  setActiveAuthBaseUrl: (baseUrl: string) => void;
  getAuth: () => Promise<NevermindAuthSnapshot>;
  signIn: () => Promise<SignInResult>;
  setActiveBaseUrl: (baseUrl: string) => void;
  warmCompatibilityCache: (baseUrl: string) => void;
  disposeAiSessions: () => Promise<unknown> | undefined;
  invalidateExtensionRootItems: () => void;
  broadcastAuthChanged: (status: { authed: true; email: string }) => void;
};

export async function switchNevermindBackendEnvironment(
  input: {
    environment: NevermindEnvironment;
    baseUrl?: string;
  },
  deps: NevermindBackendEnvironmentDeps,
) {
  const rawBaseUrl =
    input.environment === 'production'
      ? PRODUCTION_NEVERMIND_BASE_URL
      : String(input.baseUrl || '').trim();
  let parsed: URL;
  try {
    const normalized =
      input.environment === 'production'
        ? parsePublicOrigin(rawBaseUrl, 'production_api')
        : parsePublicOrigin(rawBaseUrl, 'smoke');
    parsed = new URL(normalized);
  } catch {
    try {
      const candidate = new URL(rawBaseUrl);
      if (candidate.protocol !== 'https:') {
        return { ok: false as const, message: 'Backend URL must use HTTPS.' };
      }
      if (candidate.username || candidate.password) {
        return {
          ok: false as const,
          message: 'Backend URL must not include credentials.',
        };
      }
    } catch {
      return { ok: false as const, message: 'Enter a valid backend URL.' };
    }
    return {
      ok: false as const,
      message: 'Backend URL must be a valid origin without a path.',
    };
  }
  if (parsed.protocol !== 'https:') {
    return { ok: false as const, message: 'Backend URL must use HTTPS.' };
  }
  if (parsed.username || parsed.password) {
    return {
      ok: false as const,
      message: 'Backend URL must not include credentials.',
    };
  }
  const baseUrl = parsed.origin;
  if (
    deps.isPackaged &&
    (await deps.resolvesToUnsafeAddress(parsed.hostname))
  ) {
    return {
      ok: false as const,
      message:
        'Packaged Nevermind builds cannot use localhost or private network addresses.',
    };
  }

  try {
    await deps.invalidateCompatibilityCache(baseUrl);
    const manifest = await deps.checkCompatibility(baseUrl);
    if (!manifest) {
      return {
        ok: false as const,
        message: 'That backend did not return a compatibility manifest.',
      };
    }
  } catch (error) {
    return {
      ok: false as const,
      message:
        error instanceof Error ? error.message : 'Backend validation failed.',
    };
  }

  const previous = deps.selectedEnvironment();
  deps.setSelectedEnvironment({
    environment: input.environment,
    baseUrl,
  });
  deps.scheduleSaveState();
  deps.setActiveAuthBaseUrl(baseUrl);
  await deps.invalidateCompatibilityCache(previous.baseUrl);
  const existing = await deps.getAuth();
  const result = existing
    ? { ok: true as const, auth: existing }
    : await deps.signIn();
  if (!result.ok) {
    deps.setSelectedEnvironment(previous);
    deps.scheduleSaveState();
    deps.setActiveAuthBaseUrl(previous.baseUrl);
    return {
      ok: false as const,
      message: `Sign-in failed: ${'error' in result ? result.error : 'unknown error'}`,
    };
  }
  deps.setActiveBaseUrl(result.auth.baseUrl);
  deps.setActiveAuthBaseUrl(result.auth.baseUrl);
  deps.warmCompatibilityCache(result.auth.baseUrl);
  await deps.disposeAiSessions();
  deps.invalidateExtensionRootItems();
  deps.broadcastAuthChanged({ authed: true, email: result.auth.email });
  return { ok: true as const, message: `Connected to ${baseUrl}` };
}
