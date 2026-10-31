export type PreviewAutoRun = {
  result?: unknown;
  failureMessage?: string;
  timedOut?: boolean;
};

export type GeneratedExtensionActivation = {
  filename: string;
  preview: {
    extensionId: string;
    rootItems: unknown[];
    commands?: unknown[];
    actions: unknown[];
  };
  autoRun?: PreviewAutoRun;
};

export const PREVIEW_AUTO_RUN_TIMEOUT_MS = 60_000;

type ExecutePreviewAction = (
  action: unknown,
  options: { onFailure: (error: unknown) => void },
) => Promise<unknown>;

function failureMessageFrom(error: unknown) {
  if (error instanceof Error) return error.stack || error.message;
  return String(error);
}

function withTimeout(
  execution: Promise<PreviewAutoRun>,
  timeoutMs: number,
): Promise<PreviewAutoRun> {
  let timer: NodeJS.Timeout | undefined;
  const timeout = new Promise<PreviewAutoRun>((resolve) => {
    timer = setTimeout(resolve, timeoutMs, { result: {}, timedOut: true });
  });
  return Promise.race([execution, timeout]).finally(() => clearTimeout(timer));
}

export function createPreviewAutoRunner(deps: {
  executeAction: ExecutePreviewAction;
  timeoutMs?: number;
}) {
  const timeoutMs = deps.timeoutMs ?? PREVIEW_AUTO_RUN_TIMEOUT_MS;

  return async function runPreviewAutoAction(
    activation: Pick<GeneratedExtensionActivation, 'preview'>,
  ): Promise<PreviewAutoRun | undefined> {
    const action = activation.preview.commands?.[0];
    if (!action) return undefined;
    const failures: unknown[] = [];
    const execution = deps
      .executeAction(action, { onFailure: (error) => failures.push(error) })
      .then((result): PreviewAutoRun => {
        if (failures.length === 0) return { result };
        return { result, failureMessage: failureMessageFrom(failures[0]) };
      });
    return withTimeout(execution, timeoutMs);
  };
}
