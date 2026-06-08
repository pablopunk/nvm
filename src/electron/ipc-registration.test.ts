import assert from 'node:assert/strict'
import test from 'node:test'
import { createMeasuredIpcRegistrar } from './ipc-registration'

test('createMeasuredIpcRegistrar registers measured ipc handlers', async () => {
  let registeredChannel = ''
  let registeredHandler: ((event: unknown, ...args: unknown[]) => unknown) | null = null
  const measured: Array<{ name: string; data: Record<string, unknown> }> = []
  const register = createMeasuredIpcRegistrar({
    ipcMain: {
      handle(channel, handler) {
        registeredChannel = channel
        registeredHandler = handler
      },
    },
    measure: (name, data, fn) => {
      measured.push({ name, data })
      return fn()
    },
    summarize: (value) => `summary:${String(value)}`,
  })

  register('things:do', (_event, first, second) => `${first}:${second}`)

  assert.equal(registeredChannel, 'things:do')
  assert.ok(registeredHandler)
  assert.equal(await registeredHandler({ sender: true }, 'a', 2), 'a:2')
  assert.deepEqual(measured, [{ name: 'ipc.things:do.handler', data: { args: ['summary:a', 'summary:2'], alwaysLog: true } }])
})
