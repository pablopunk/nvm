import assert from 'node:assert/strict'
import test from 'node:test'
import { extensionPermissionCapabilities, filterWebviewPermissionsForExtension, hasExtensionPermission, isInternalExtension, markInternalExtension, permissionDeniedError } from './extension-permissions'

test('only host-marked internal extensions are trusted when they omit explicit permissions', () => {
  const extension = markInternalExtension({ id: 'nevermind.system' })

  assert.equal(isInternalExtension(extension), true)
  assert.equal(hasExtensionPermission(extension, 'system'), true)
  assert.equal(hasExtensionPermission(extension, 'desktop.files'), true)
  assert.equal(isInternalExtension({ id: 'nevermind.spoof' }), false)
  assert.equal(hasExtensionPermission({ id: 'nevermind.spoof' }, 'system'), false)
})

test('explicit permission arrays are authoritative even for internal extensions', () => {
  assert.equal(hasExtensionPermission(markInternalExtension({ id: 'nevermind.fixture', permissions: [] }), 'system'), false)
  assert.equal(hasExtensionPermission({ id: 'third.party', permissions: ['ai'] }, 'ai'), true)
  assert.equal(hasExtensionPermission({ id: 'third.party', permissions: ['ai'] }, 'system'), false)
})

test('extensionPermissionCapabilities exposes all host context gates', () => {
  assert.deepEqual(extensionPermissionCapabilities({ id: 'third.party', permissions: ['desktop.apps', 'clipboard.history', 'extensions.ownership'] }), {
    canUseDesktopApps: true,
    canUseDesktopFiles: false,
    canUseClipboard: true,
    canUseSystem: false,
    canUseOcr: false,
    canUseUpdates: false,
    canUseShortcuts: false,
    canUseAi: false,
    canWriteSettings: false,
    canManageExtensionOwnership: true,
  })
})

test('filterWebviewPermissionsForExtension gates delegated iframe permissions by host manifest permissions', () => {
  assert.deepEqual(filterWebviewPermissionsForExtension({ id: 'third.party', permissions: ['camera', 'clipboard.history'] }, ['autoplay', 'camera', 'microphone', 'display-capture', 'clipboard-read', 'clipboard-write', 'unknown']), [
    'autoplay',
    'camera',
    'microphone',
    'clipboard-read',
    'clipboard-write',
  ])
  assert.deepEqual(filterWebviewPermissionsForExtension({ id: 'third.party', permissions: [] }, ['autoplay', 'camera', 'clipboard-read']), ['autoplay'])
  assert.deepEqual(filterWebviewPermissionsForExtension({ id: 'third.party', permissions: ['desktop.files'] }, ['display-capture']), ['display-capture'])
})

test('permissionDeniedError uses stable host error text', () => {
  assert.equal(permissionDeniedError('system').message, 'Extension is missing required permission: system')
})
