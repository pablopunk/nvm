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

function renderExtensionView(
  view: CommandView,
  aiChatOverrides: Partial<ExtensionViewRendererProps['aiChat']> = {},
) {
  const props: ExtensionViewRendererProps = {
    view,
    aiChat: {
      messages: [],
      input: '',
      setInput: () => {},
      attachments: [],
      attaching: false,
      attachmentError: null,
      attachImageFiles: async () => false,
      removeAttachment: () => {},
      busy: false,
      limit: null,
      creditNotice: null,
      inputRef: React.createRef<HTMLTextAreaElement>(),
      messagesRef: React.createRef<HTMLDivElement>(),
      resizeInput: () => {},
      ...aiChatOverrides,
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

test('renders sent and pending AI chat image attachments', () => {
  const html = renderExtensionView(
    {
      type: 'chat',
      title: 'Images',
      aiChat: true,
      messages: [],
    },
    {
      messages: [
        {
          role: 'user',
          content: 'What is this?',
          images: [{ url: 'nvm-file://local/sent.png', alt: 'Sent image' }],
        },
      ],
      attachments: [
        {
          id: 'pending',
          data: 'aW1hZ2U=',
          mimeType: 'image/png',
          byteLength: 5,
          name: 'Pending image',
          previewUrl: 'data:image/png;base64,aW1hZ2U=',
        },
      ],
    },
  );

  assert.match(html, /Sent image/);
  assert.match(html, /Pending image/);
  assert.match(html, /Remove Pending image/);
});

test('renders forms as a grouped keyboard-first surface', () => {
  const html = renderExtensionView({
    type: 'form',
    title: 'Account',
    fields: [
      { id: 'name', label: 'Name', type: 'text', description: 'Your name' },
      {
        id: 'roles',
        label: 'Roles',
        type: 'multiselect',
        options: [
          { title: 'Author', value: 'author' },
          { title: 'Reviewer', value: 'reviewer' },
        ],
      },
    ],
    submitAction: {
      type: 'runExtensionAction',
      title: 'Save Account',
      handlerId: 'save-account',
    },
  });

  assert.match(html, /class="extensionView formView"/);
  assert.match(html, /aria-keyshortcuts="Meta\+Enter Control\+Enter"/);
  assert.match(html, /class="formFields"/);
  assert.match(html, /for="form-field-control-name"/);
  assert.match(html, /id="form-field-description-name"/);
  assert.match(html, /class="formMultiselect"/);
  assert.match(html, /<kbd>Tab<\/kbd> Move between fields/);
  assert.match(html, /<span>Save Account<\/span><kbd>⌘↵<\/kbd>/);
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
          action: {
            type: 'checkForUpdates',
            title: 'Check for Update',
            executionId: 'trusted-update-action',
          },
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

  assert.doesNotMatch(html, /keyHints selectedOnlyEnter/);
  assert.match(html, /<span class="shortcutHint">⌘Y<\/span>/);
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

test('renders an AI limit with a structured dashboard action', () => {
  const html = renderToStaticMarkup(
    <Command>
      <NevermindLimitGate
        limit={{
          kind: 'insufficient_credits',
          title: 'Credits needed',
          message: 'Open your dashboard to review your account.',
          actionTitle: 'Open Dashboard',
          action: {
            type: 'openUrl',
            title: 'Open Dashboard',
            url: 'https://www.nvm.fyi/dashboard',
            executionId: 'trusted-dashboard-action',
          },
        }}
        runAction={() => {}}
      />
    </Command>,
  );

  assert.match(html, /role="status"/);
  assert.match(html, /Credits needed/);
  assert.match(html, /Open your dashboard to review your account\./);
  assert.match(html, /Open Dashboard/);
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
