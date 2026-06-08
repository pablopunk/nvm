import assert from 'node:assert/strict'
import { test } from 'node:test'
import { safeExternalUrl } from './url-utils'

test('safeExternalUrl accepts normal external browser URLs', () => {
  assert.equal(safeExternalUrl('https://nvm.fyi/path?q=1'), 'https://nvm.fyi/path?q=1')
  assert.equal(safeExternalUrl('mailto:pablo@example.com'), 'mailto:pablo@example.com')
})

test('safeExternalUrl rejects privileged or malformed URLs', () => {
  assert.equal(safeExternalUrl('javascript:alert(1)'), null)
  assert.equal(safeExternalUrl('file:///etc/passwd'), null)
  assert.equal(safeExternalUrl('x-apple.systempreferences:'), null)
  assert.equal(safeExternalUrl(''), null)
})
