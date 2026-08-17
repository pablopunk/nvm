import assert from 'node:assert/strict';
import { test } from 'node:test';
import { Command } from 'cmdk';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import {
  EXTENSION_WEBVIEW_ALLOW,
  EXTENSION_WEBVIEW_SANDBOX,
  ExtensionViewRenderer,
  type ExtensionViewRendererProps,
  extensionWebviewAllow,
  NevermindLimitGate,
} from './extension-view';
import { feedbackView } from './feedback';
import type { CommandAction, CommandView } from './model';
import { MarkdownContent } from './ui';
import {
  nextNavigationState,
  previousNavigationState,
} from './use-extension-navigation';

function renderExtensionView(view: CommandView) {
  const props: ExtensionViewRendererProps = {
    view,
    aiChat: {
      messages: [],
      input: '',
      setInput: () => {},
      busy: false,
      limit: null,
      creditNotice: null,
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
  };
  return renderToStaticMarkup(
    <Command>
      <ExtensionViewRenderer {...props} />
    </Command>,
  );
}

test('renders indicator snapshots newest first with stable progress', () => {
  const html = renderExtensionView({
    type: 'indicator-stack',
    title: 'Status',
    entries: [
      {
        id: 'new',
        sequence: 2,
        title: 'Dictation',
        label: 'Listening',
        status: 'recording',
      },
      {
        id: 'old',
        sequence: 1,
        title: 'Dictation',
        label: 'Waiting for AirPods...',
        status: 'loading',
        value: 1,
        total: 4,
      },
    ],
  });

  assert.ok(html.indexOf('Listening') < html.indexOf('Waiting for AirPods'));
  assert.match(html, /data-status="recording"/);
  assert.match(html, /25%/);
  assert.match(html, /aria-live="polite"/);
});

test('renders unsupported-client update UI with structured updater action', () => {
  const actions: CommandAction[] = [];
  const html = renderToStaticMarkup(
    <Command>
      <NevermindLimitGate
        limit={{
          kind: 'unsupported_client',
          title: 'Update Nevermind',
          message: 'This version is no longer supported by the backend.',
          actionTitle: 'Check for Update',
          action: { type: 'checkForUpdates', title: 'Check for Update' },
        }}
        runAction={(action) => actions.push(action)}
      />
    </Command>,
  );

  assert.match(html, /role="status"/);
  assert.match(html, /Update Nevermind/);
  assert.match(html, /This version is no longer supported by the backend\./);
  assert.match(html, /Check for Update/);
  assert.doesNotMatch(html, /Open Dashboard/);
});

test('renders feedback as a cmdk list with an accessible recovery action', () => {
  const html = renderExtensionView(
    feedbackView({
      id: 'uninstall-unavailable',
      title: 'Uninstall unavailable',
      message: 'This app cannot be uninstalled safely.',
      tone: 'error',
    }),
  );

  assert.match(html, /Uninstall unavailable/);
  assert.match(html, /This app cannot be uninstalled safely\./);
  assert.match(html, /Back/);
  assert.match(html, /aria-disabled="true"/);
});

test('does not render the legacy thin loading bar', () => {
  const activeLoading = renderExtensionView({
    type: 'list',
    title: 'Running action',
    isLoading: true,
    items: [{ id: 'one', title: 'One' }],
  });
  assert.doesNotMatch(activeLoading, /viewLoadingBar/);
});

test('loading views do not render an inline placeholder', () => {
  const loadingView = renderExtensionView({
    type: 'list',
    title: 'Loading fixture',
    isLoading: true,
    emptyView: { title: 'No items yet' },
    items: [],
  });

  assert.doesNotMatch(loadingView, /No items yet/);
  assert.doesNotMatch(loadingView, /loadingSpinner|spinnerIcon/);
});

test('renders an opted-in side preview for the selected list item', () => {
  const html = renderExtensionView({
    type: 'list',
    presentation: 'side-preview',
    title: 'Clipboard History',
    selectedItemId: 'one',
    items: [
      {
        id: 'one',
        title: 'Clipboard text',
        detail: { text: 'Preview content' },
      },
    ],
  });

  assert.match(html, /extensionListWithDetail/);
  assert.match(html, /Preview content/);
  assert.doesNotMatch(html, /extensionDetailHeader/);
});

test('renders side-preview media and text details', () => {
  const html = renderExtensionView({
    type: 'list',
    presentation: 'side-preview',
    title: 'Clipboard History',
    items: [
      {
        id: 'video',
        title: 'Clipboard video',
        detail: {
          image: 'thumb://video',
          video: 'file://video.mp4',
        },
      },
    ],
  });

  assert.match(html, /class="extensionDetailMedia"/);
  assert.match(html, /extensionDetailHeader/);
  assert.match(html, /file:\/\/video\.mp4/);
});

test('keeps local action shortcuts selected-only in extension lists', () => {
  const html = renderExtensionView({
    type: 'list',
    title: 'Clipboard History',
    items: [
      {
        id: 'one',
        title: 'One',
        actions: [{ title: 'Paste', type: 'pasteText', shortcut: 'Command+Y' }],
      },
      {
        id: 'two',
        title: 'Two',
        actions: [{ title: 'Paste', type: 'pasteText', shortcut: 'Command+Y' }],
      },
    ],
  });

  assert.equal((html.match(/selectedOnlyEnter/g) || []).length, 2);
});

test('keeps durable global shortcuts visible in extension lists', () => {
  const html = renderExtensionView({
    type: 'list',
    title: 'Actions',
    items: [
      {
        id: 'one',
        title: 'One',
        persistentAction: {
          type: 'runExtensionRegisteredAction',
          title: 'One',
          shortcut: 'Command+Y',
          shortcutScope: 'global',
        },
        actions: [{ title: 'Run', type: 'runExtensionAction' }],
      },
    ],
  });

  assert.doesNotMatch(html, /selectedOnlyEnter/);
  assert.match(html, /⌘Y/);
});

test('renders a native glyph as a grid tile visual', () => {
  const html = renderExtensionView({
    type: 'grid',
    title: 'Characters',
    items: [{ id: 'sparkles', title: 'Sparkles', glyph: '✨' }],
  });

  assert.match(html, /tileIcon tileGlyph/);
  assert.match(html, /aria-hidden="true">✨<\/span>/);
  assert.match(html, />Sparkles<\/strong>/);
});

test('bounds rendered grid items without removing them from the searchable view', () => {
  const html = renderExtensionView({
    type: 'grid',
    title: 'Characters',
    maxVisibleItems: 1,
    items: [
      { id: 'one', title: 'One', glyph: '1' },
      { id: 'two', title: 'Two', glyph: '2' },
    ],
  });

  assert.match(html, />One<\/strong>/);
  assert.doesNotMatch(html, />Two<\/strong>/);
});

test('renders markdown headings with host typography hooks', () => {
  const html = renderToStaticMarkup(
    <MarkdownContent
      content={'# Preview Fixture\n\nA compact preview body.'}
    />,
  );

  assert.match(
    html,
    /<h1 class="markdownHeading markdownHeading1">Preview Fixture<\/h1>/,
  );
});

test('nested navigation preserves the parent view when pushing a child', () => {
  const root: CommandView = {
    id: 'ai-chats',
    type: 'list',
    title: 'AI Chats',
    items: [],
  };
  const child: CommandView = {
    id: 'chat:1',
    type: 'chat',
    title: 'Chat 1',
    aiChat: true,
  };

  const pushed = nextNavigationState(
    { view: root, backStack: [] },
    child,
    'push',
  );
  assert.equal(pushed.view, child);
  assert.deepEqual(pushed.backStack, [root]);

  const popped = previousNavigationState(pushed);
  assert.equal(popped.didPop, true);
  assert.equal(popped.state.view, root);
  assert.deepEqual(popped.state.backStack, []);
});

test('root navigation intentionally clears nested history', () => {
  const parent: CommandView = {
    id: 'parent',
    type: 'list',
    title: 'Parent',
    items: [],
  };
  const current: CommandView = {
    id: 'current',
    type: 'list',
    title: 'Current',
    items: [],
  };
  const nextRoot: CommandView = {
    id: 'root',
    type: 'list',
    title: 'Root',
    items: [],
  };

  const rooted = nextNavigationState(
    { view: current, backStack: [parent] },
    nextRoot,
    'root',
  );
  assert.equal(rooted.view, nextRoot);
  assert.deepEqual(rooted.backStack, []);
});

test('renders deprecation-warning UI with dashboard fallback action', () => {
  const html = renderToStaticMarkup(
    <Command>
      <NevermindLimitGate
        limit={{
          kind: 'deprecation_warning',
          title: 'Backend API deprecation',
          message:
            'This API contract will sunset soon. Review the migration path.',
          actionTitle: 'Review migration',
          dashboardUrl: 'https://www.nvm.fyi/dashboard',
        }}
        runAction={() => {}}
      />
    </Command>,
  );

  assert.match(html, /role="status"/);
  assert.match(html, /Backend API deprecation/);
  assert.match(
    html,
    /This API contract will sunset soon\. Review the migration path\./,
  );
  assert.match(html, /Review migration/);
});

test('extension webview iframe keeps scripts isolated from app origin and privileged permissions', () => {
  const html = renderExtensionView({
    type: 'webview',
    title: 'Sandboxed HTML',
    html: '<form><script>document.body.dataset.ready = "1"</script></form>',
  });

  assert.match(html, /<iframe/);
  assert.match(html, new RegExp(`sandbox="${EXTENSION_WEBVIEW_SANDBOX}"`));
  assert.match(html, new RegExp(`allow="${EXTENSION_WEBVIEW_ALLOW}"`));
  assert.doesNotMatch(html, /allow-same-origin/);
  assert.doesNotMatch(
    html,
    /camera|microphone|display-capture|clipboard-read|clipboard-write/,
  );
});

test('extension webview privileged iframe permissions are explicit and allowlisted', () => {
  assert.equal(
    extensionWebviewAllow(['autoplay', 'camera', 'camera', 'bad-permission']),
    'autoplay; camera',
  );

  const html = renderExtensionView({
    type: 'webview',
    title: 'Camera HTML',
    html: '<main>camera</main>',
    webviewPermissions: ['camera', 'microphone'],
  });

  assert.match(html, /allow="camera; microphone"/);
  assert.doesNotMatch(html, /allow-same-origin/);
  assert.doesNotMatch(html, /display-capture|clipboard-read|clipboard-write/);
});
