import type { PiRuntimeModuleResolution } from './pi-runtime';

export type PackagedPiSessionAi = {
  session: (
    sessionId: string,
    options: { model: 'fast'; toolMode: 'conversation' },
  ) => {
    ask: (
      prompt: string,
      options: { onEvent: (event: { type: string }) => void },
    ) => Promise<unknown>;
  };
  disposeAllSessions: () => Promise<void>;
};

export type PackagedPiSessionFixtureDeps = {
  enabled: boolean;
  isPackaged: () => boolean;
  appPath: () => string;
  initializeAi: () => Promise<unknown>;
  getAi: () => PackagedPiSessionAi | null;
  resolveModules: () => PiRuntimeModuleResolution;
};

export function createPackagedPiSessionFixture(
  deps: PackagedPiSessionFixtureDeps,
) {
  const { enabled, isPackaged, appPath, initializeAi, getAi, resolveModules } =
    deps;

  return async function runPackagedPiSessionFixture() {
    if (!enabled || !isPackaged())
      throw new Error(
        'Packaged Pi session is available only in packaged test mode',
      );

    const terminalEvents: string[] = [];
    let response: unknown;
    let errorName: string | undefined;
    let resolvedModules: Partial<PiRuntimeModuleResolution> = {};
    let sessionAi: PackagedPiSessionAi | null = null;
    let sessionStarted = false;
    let sessionDisposed = false;
    try {
      let ai = getAi();
      if (!ai) {
        await initializeAi();
        ai = getAi();
      }
      if (!ai) throw new Error('Nevermind Pi adapter failed to initialize');
      sessionAi = ai;
      const session = ai.session('packaged-electron-pi-smoke', {
        model: 'fast',
        toolMode: 'conversation',
      });
      sessionStarted = true;
      response = await session.ask(
        'Reply with exactly PACKAGED_PI_ELECTRON_SESSION_OK. Do not call tools.',
        {
          onEvent: (event) => {
            if (['done', 'error', 'aborted'].includes(event.type))
              terminalEvents.push(event.type);
          },
        },
      );
    } catch (error) {
      errorName = error instanceof Error ? error.name : 'UnknownError';
    } finally {
      try {
        resolvedModules = resolveModules();
      } catch (error) {
        errorName ||=
          error instanceof Error ? error.name : 'ModuleResolutionError';
      }
      if (sessionStarted) {
        if (!sessionAi) {
          errorName ||= 'SessionDisposeError';
        } else {
          try {
            await sessionAi.disposeAllSessions();
            sessionDisposed = true;
          } catch {
            errorName ||= 'SessionDisposeError';
          }
        }
      }
    }

    return {
      appIsPackaged: isPackaged(),
      appPath: appPath(),
      terminalEvents,
      responseMatches: response === 'PACKAGED_PI_ELECTRON_SESSION_OK',
      resolvedModules,
      sessionDisposed,
      ...(errorName ? { errorName } : {}),
    };
  };
}
