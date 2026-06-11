import assert from 'node:assert/strict'
import test from 'node:test'
import { createDataLoaderHandle, createViewLoaderRegistry, isLoaderHandle, normalizeLoaderItems, resolveLoaderEmptyView } from './data-loader'

test('ctx.data.loader returns an opaque handle and normalizeLoaderItems strips it', async () => {
  const loader = createDataLoaderHandle(async () => [{ id: 'a' }], { retry: true })

  assert.equal(loader._loader, true)
  assert.equal(isLoaderHandle(loader), true)
  assert.equal(typeof loader._fn, 'function')
  assert.equal(loader._retry, true)
  assert.deepEqual(normalizeLoaderItems(loader), [])
})

test('loader registry hydrates normalized items', async () => {
  const payloads: Array<{ viewId: string; payload: Record<string, unknown> }> = []
  const registry = createViewLoaderRegistry({
    sendHydrate: (viewId, payload) => payloads.push({ viewId, payload }),
    normalizeItems: (items) => items.map((item) => ({ ...item, normalized: true })),
  })

  registry.register('view:1', createDataLoaderHandle(async () => [{ id: 'a', title: 'A' }]), { extension: { id: 'test' } })
  assert.equal(registry.has('view:1'), true)

  await registry.spawn('view:1')

  assert.equal(registry.has('view:1'), false)
  assert.deepEqual(payloads, [{
    viewId: 'view:1',
    payload: {
      items: [{ id: 'a', title: 'A', normalized: true }],
      isLoading: false,
    },
  }])
})

test('views with loaders get a default empty state when one is not provided', async () => {
  const loader = createDataLoaderHandle(async () => [])

  assert.deepEqual(resolveLoaderEmptyView(undefined, loader), { title: 'No items', subtitle: '' })
  assert.deepEqual(resolveLoaderEmptyView({ title: 'Custom', subtitle: 'Empty' }, loader), { title: 'Custom', subtitle: 'Empty' })
  assert.equal(resolveLoaderEmptyView(undefined), undefined)
})

test('loader errors preserve entry for retry and retry re-runs after re-registration', async () => {
  const payloads: Array<Record<string, unknown>> = []
  const warnings: Array<{ viewId: string; message: string }> = []
  const registry = createViewLoaderRegistry({
    sendHydrate: (_viewId, payload) => payloads.push(payload),
    normalizeItems: (items) => items,
    warn: (viewId, message) => warnings.push({ viewId, message }),
  })

  // Non-retry: entry is cleaned up after error
  registry.register('view:no-retry', createDataLoaderHandle(async () => { throw new Error('boom') }), null)
  await registry.spawn('view:no-retry')
  assert.equal(registry.has('view:no-retry'), false)

  // Retry-enabled: entry is preserved after error so retry can re-run
  registry.register('view:retry', createDataLoaderHandle(async () => { throw new Error('retry me') }, { retry: true }), null)
  await registry.spawn('view:retry')
  assert.equal(registry.has('view:retry'), true)

  // Re-register with a passing loader to simulate a successful retry
  registry.register('view:retry', createDataLoaderHandle(async () => [{ id: 'retried' }], { retry: true }), null)
  await registry.retry('view:retry')
  assert.equal(registry.has('view:retry'), false)

  assert.deepEqual(payloads, [
    { error: { message: 'boom' }, retry: false },
    { error: { message: 'retry me' }, retry: true },
    { items: [{ id: 'retried' }], isLoading: false },
  ])
  assert.deepEqual(warnings, [
    { viewId: 'view:no-retry', message: 'boom' },
    { viewId: 'view:retry', message: 'retry me' },
  ])
})
