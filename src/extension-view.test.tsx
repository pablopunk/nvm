import assert from 'node:assert/strict'
import { test } from 'node:test'
import React from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { Command } from 'cmdk'
import { EXTENSION_WEBVIEW_ALLOW, EXTENSION_WEBVIEW_SANDBOX, ExtensionViewRenderer, NevermindLimitGate, type ExtensionViewRendererProps } from './extension-view'
import { nextNavigationState, previousNavigationState } from './use-extension-navigation'
import type { CommandAction, CommandView } from './model'

function renderExtensionView(view: CommandView) {
  const props: ExtensionViewRendererProps = {
    view,
    aiChat: {
      messages: [],
      input: '',
      setInput: () => {},
      busy: false,
      limit: null,
      inputRef: React.createRef<HTMLTextAreaElement>(),
      messagesRef: React.createRef<HTMLDivElement>(),
      resizeInput: () => {},
    },
    nevermindAuthed: null,
    onSignInToNevermind: () => {},
    formValues: {},
    setFormValues: () => {},
    filterItems: (items) => items || [],
    filterSections: (currentView) => currentView.sections,
    renderMarkdown: (content) => content,
    renderActionPanel: () => null,
    actionPanelRows: () => [],
    renderRootIcon: () => null,
    runDefaultAction: () => {},
    runAction: () => {},
    sendAiPrompt: () => {},
    abortAiChat: () => {},
    dragPathForItem: () => null,
    startItemDrag: () => {},
  }
  return renderToStaticMarkup(<Command><ExtensionViewRenderer {...props} /></Command>)
}

test('renders unsupported-client update UI with structured updater action', () => {
  const actions: CommandAction[] = []
  const html = renderToStaticMarkup(<Command><NevermindLimitGate
    limit={{
      kind: 'unsupported_client',
      title: 'Update Nevermind',
      message: 'This version is no longer supported by the backend.',
      actionTitle: 'Check for Update',
      action: { type: 'checkForUpdates', title: 'Check for Update' },
    }}
    runAction={(action) => actions.push(action)}
  /></Command>)

  assert.match(html, /role="status"/)
  assert.match(html, /Update Nevermind/)
  assert.match(html, /This version is no longer supported by the backend\./)
  assert.match(html, /Check for Update/)
  assert.doesNotMatch(html, /Open Dashboard/)
})

test('nested navigation preserves the parent view when pushing a child', () => {
  const root: CommandView = { id: 'ai-chats', type: 'list', title: 'AI Chats', items: [] }
  const child: CommandView = { id: 'chat:1', type: 'chat', title: 'Chat 1', aiChat: true }

  const pushed = nextNavigationState({ view: root, backStack: [] }, child, 'push')
  assert.equal(pushed.view, child)
  assert.deepEqual(pushed.backStack, [root])

  const popped = previousNavigationState(pushed)
  assert.equal(popped.didPop, true)
  assert.equal(popped.state.view, root)
  assert.deepEqual(popped.state.backStack, [])
})

test('root navigation intentionally clears nested history', () => {
  const parent: CommandView = { id: 'parent', type: 'list', title: 'Parent', items: [] }
  const current: CommandView = { id: 'current', type: 'list', title: 'Current', items: [] }
  const nextRoot: CommandView = { id: 'root', type: 'list', title: 'Root', items: [] }

  const rooted = nextNavigationState({ view: current, backStack: [parent] }, nextRoot, 'root')
  assert.equal(rooted.view, nextRoot)
  assert.deepEqual(rooted.backStack, [])
})

test('renders deprecation-warning UI with dashboard fallback action', () => {
  const html = renderToStaticMarkup(<Command><NevermindLimitGate
    limit={{
      kind: 'deprecation_warning',
      title: 'Backend API deprecation',
      message: 'This API contract will sunset soon. Review the migration path.',
      actionTitle: 'Review migration',
      dashboardUrl: 'https://nvm.fyi/dashboard',
    }}
    runAction={() => {}}
  /></Command>)

  assert.match(html, /role="status"/)
  assert.match(html, /Backend API deprecation/)
  assert.match(html, /This API contract will sunset soon\. Review the migration path\./)
  assert.match(html, /Review migration/)
})

test('extension webview iframe keeps scripts isolated from app origin and privileged permissions', () => {
  const html = renderExtensionView({
    type: 'webview',
    title: 'Sandboxed HTML',
    html: '<form><script>document.body.dataset.ready = "1"</script></form>',
  })

  assert.match(html, /<iframe/)
  assert.match(html, new RegExp(`sandbox="${EXTENSION_WEBVIEW_SANDBOX}"`))
  assert.match(html, new RegExp(`allow="${EXTENSION_WEBVIEW_ALLOW}"`))
  assert.doesNotMatch(html, /allow-same-origin/)
  assert.doesNotMatch(html, /camera|microphone|display-capture|clipboard-read|clipboard-write/)
})
