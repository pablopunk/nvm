export type IpcMainLike = {
  handle(channel: string, handler: (event: unknown, ...args: unknown[]) => unknown): void
}

export type MeasuredIpcRegistrarDeps = {
  ipcMain: IpcMainLike
  measure: <T>(name: string, data: Record<string, unknown>, fn: () => T | Promise<T>) => T | Promise<T>
  summarize: (value: unknown) => unknown
}

export function createMeasuredIpcRegistrar({ ipcMain, measure, summarize }: MeasuredIpcRegistrarDeps) {
  return function ipcHandleMeasured(channel: string, handler: (event: any, ...args: any[]) => unknown) {
    ipcMain.handle(channel, (event, ...args) => measure(`ipc.${channel}.handler`, { args: args.map(summarize), alwaysLog: true }, () => handler(event, ...args)))
  }
}
