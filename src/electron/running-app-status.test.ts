import assert from 'node:assert/strict'
import test from 'node:test'
import { createRunningAppStatusService, type RunningAppCandidate } from './running-app-status'

function createDeferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

test('returns cached running paths immediately while refreshing stale snapshots', async () => {
  let now = 0
  const firstRefresh = createDeferred<Set<string>>()
  const secondRefresh = createDeferred<Set<string>>()
  const detections = [firstRefresh, secondRefresh]
  let detectCount = 0
  let changeCount = 0
  const candidates: RunningAppCandidate[] = [{ path: '/Applications/Notes.app' }]
  const service = createRunningAppStatusService({
    ttlMs: 100,
    platform: 'darwin',
    now: () => now,
    getCandidates: () => candidates,
    detectRunningAppPaths: () => detections[detectCount++].promise,
    notifyChanged: () => {
      changeCount += 1
    },
  })

  const initial = service.refresh('initial')
  firstRefresh.resolve(new Set(['/applications/notes.app']))
  await initial

  now = 200
  const openPaths = await service.getForRenderer(['/Applications/Notes.app'])

  assert.deepEqual(openPaths, ['/Applications/Notes.app'])
  assert.equal(detectCount, 2)
  assert.equal(changeCount, 1)

  secondRefresh.resolve(new Set(['/applications/notes.app', '/applications/mail.app']))
  await service.refresh('join-in-flight')
  assert.equal(changeCount, 2)
})

test('does not block renderer requests when no snapshot exists', async () => {
  const refresh = createDeferred<Set<string>>()
  let detectCount = 0
  const service = createRunningAppStatusService({
    ttlMs: 100,
    platform: 'darwin',
    getCandidates: () => [{ path: '/Applications/Notes.app' }],
    detectRunningAppPaths: () => {
      detectCount += 1
      return refresh.promise
    },
    notifyChanged: () => {},
  })

  const openPaths = await service.getForRenderer(['/Applications/Notes.app'])

  assert.deepEqual(openPaths, [])
  assert.equal(detectCount, 1)
  refresh.resolve(new Set(['/applications/notes.app']))
  await service.refresh('join-in-flight')
})

test('normalizes detected running paths before storing snapshots', async () => {
  const service = createRunningAppStatusService({
    ttlMs: 100,
    platform: 'darwin',
    getCandidates: () => [{ path: '/Applications/Notes.app' }],
    detectRunningAppPaths: async () => new Set(['/Applications/NOTES.app']),
    notifyChanged: () => {},
  })

  await service.refresh('initial')

  assert.deepEqual(await service.getForRenderer(['/Applications/Notes.app']), ['/Applications/Notes.app'])
})

test('dedupes in-flight refreshes and caps renderer requested paths', async () => {
  let detectCount = 0
  const service = createRunningAppStatusService({
    ttlMs: 100,
    platform: 'linux',
    getCandidates: () => [{ path: '/usr/bin/app' }],
    detectRunningAppPaths: async () => {
      detectCount += 1
      return new Set(['/usr/bin/app'])
    },
    notifyChanged: () => {},
  })

  service.scheduleRefresh('one')
  service.scheduleRefresh('two')
  await service.refresh('three')
  const requested = Array.from({ length: 35 }, (_item, index) => index === 0 ? '/usr/bin/app' : `/tmp/${index}`)

  assert.equal(detectCount, 1)
  assert.deepEqual(await service.getForRenderer(requested), ['/usr/bin/app'])
})
