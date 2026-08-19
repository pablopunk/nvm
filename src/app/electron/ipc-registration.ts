export type IpcMainLike = {
  handle(
    channel: string,
    handler: (event: unknown, ...args: unknown[]) => unknown,
  ): void;
};

export type MeasuredIpcRegistrarDeps = {
  ipcMain: IpcMainLike;
  measure: <T>(
    name: string,
    data: Record<string, unknown>,
    fn: () => T | Promise<T>,
  ) => T | Promise<T>;
  summarize: (value: unknown) => unknown;
  trace?: {
    run<T>(
      operation: string,
      data: Record<string, unknown>,
      fn: () => T | Promise<T>,
      context?: { traceId: string },
    ): T | Promise<T>;
  };
};

function traceIdFromIpcArgs(channel: string, args: unknown[]) {
  for (const value of args) {
    if (!value || typeof value !== 'object') continue;
    const traceId = (value as { traceId?: unknown }).traceId;
    if (typeof traceId === 'string' && traceId) return traceId;
  }
  if (channel === 'ai:chat:send') {
    const traceId = args[2];
    if (typeof traceId === 'string' && traceId) return traceId;
  }
  return undefined;
}

export function createMeasuredIpcRegistrar({
  ipcMain,
  measure,
  summarize,
  trace,
}: MeasuredIpcRegistrarDeps) {
  return function ipcHandleMeasured(
    channel: string,
    handler: (event: any, ...args: any[]) => unknown,
  ) {
    ipcMain.handle(channel, (event, ...args) => {
      const invoke = () =>
        measure(
          `ipc.${channel}.handler`,
          { args: args.map(summarize), alwaysLog: true },
          () => handler(event, ...args),
        );
      const traceId = traceIdFromIpcArgs(channel, args);
      return trace && channel !== 'logs:write'
        ? trace.run(
            `ipc.${channel}`,
            { argumentCount: args.length },
            invoke,
            traceId ? { traceId } : undefined,
          )
        : invoke();
    });
  };
}
