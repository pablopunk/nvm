import assert from 'node:assert/strict'
import test from 'node:test'
import { installExternalNavigationPolicy, isTrustedAppPage, isTrustedExtensionWindowPage } from './window-navigation-policy'

function createFakeWindow() {
  let openHandler: ((details: { url: string }) => { action: 'allow' | 'deny' }) | null = null
  let navigateHandler: ((event: { preventDefault: () => void }, url: string) => void) | null = null
  return {
    win: {
      webContents: {
        setWindowOpenHandler(handler: (details: { url: string }) => { action: 'allow' | 'deny' }) {
          openHandler = handler
        },
        on(event: 'will-navigate', handler: (event: { preventDefault: () => void }, url: string) => void) {
          assert.equal(event, 'will-navigate')
          navigateHandler = handler
        },
      },
    },
    open(url: string) {
      assert.ok(openHandler)
      return openHandler({ url })
    },
    navigate(url: string) {
      assert.ok(navigateHandler)
      let prevented = false
      navigateHandler({ preventDefault: () => { prevented = true } }, url)
      return prevented
    },
  }
}

test('identifies trusted app and extension window pages', () => {
  const rendererIndexPath = '/app/index.html'
  assert.equal(isTrustedAppPage('file:///app/index.html', false, '', rendererIndexPath), true)
  assert.equal(isTrustedAppPage('file:///tmp/evil.html', false, '', rendererIndexPath), false)
  assert.equal(isTrustedAppPage('file:///app/index.html', false), false)
  assert.equal(isTrustedAppPage('http://localhost:5173/', true, 'http://localhost:5173/'), true)
  assert.equal(isTrustedAppPage('http://localhost:5173.evil.test/', true, 'http://localhost:5173/'), false)
  assert.equal(isTrustedAppPage('https://example.com/', true, 'http://localhost:5173/'), false)

  assert.equal(isTrustedExtensionWindowPage('file:///app/index.html?extensionWindowId=abc%20123', 'abc 123', false, '', rendererIndexPath), true)
  assert.equal(isTrustedExtensionWindowPage('file:///app/index.html', 'abc 123', false, '', rendererIndexPath), false)
  assert.equal(isTrustedExtensionWindowPage('file:///tmp/evil.html?extensionWindowId=abc%20123', 'abc 123', false, '', rendererIndexPath), false)
  assert.equal(isTrustedExtensionWindowPage('http://localhost:5173/?extensionWindowId=abc%20123', 'abc 123', true, 'http://localhost:5173/'), true)
  assert.equal(isTrustedExtensionWindowPage('http://localhost:5173.evil.test/?extensionWindowId=abc%20123', 'abc 123', true, 'http://localhost:5173/'), false)
  assert.equal(isTrustedExtensionWindowPage('http://localhost:5173/?extensionWindowId=other', 'abc 123', true, 'http://localhost:5173/'), false)
})

test('denies popups and routes untrusted navigation externally', () => {
  const fake = createFakeWindow()
  const opened: string[] = []
  installExternalNavigationPolicy(fake.win, (url) => url.startsWith('file:'), async (url) => {
    opened.push(url)
    return true
  })

  assert.deepEqual(fake.open('https://example.com/'), { action: 'deny' })
  assert.equal(fake.navigate('file:///app/index.html'), false)
  assert.equal(fake.navigate('https://example.com/'), true)
  assert.deepEqual(opened, ['https://example.com/', 'https://example.com/'])
})
