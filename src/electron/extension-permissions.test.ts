import assert from 'node:assert/strict'
import test from 'node:test'
import { extensionPermissionCapabilities, hasExtensionPermission, isInternalExtension, permissionDeniedError } from './extension-permissions'

test('internal extensions are trusted when they omit explicit permissions', () => {
  const extension = { id: 'nevermind.system' }

  assert.equal(isInternalExtension(extension), true)
  assert.equal(hasExtensionPermission(extension, 'system'), true)
  assert.equal(hasExtensionPermission(extension, 'desktop.files'), true)
})

test('explicit permission arrays are authoritative even for internal extensions', () => {
  assert.equal(hasExtensionPermission({ id: 'nevermind.fixture', permissions: [] }, 'system'), false)
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

test('permissionDeniedError uses stable host error text', () => {
  assert.equal(permissionDeniedError('system').message, 'Extension is missing required permission: system')
})
