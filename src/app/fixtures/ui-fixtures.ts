// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.

import type { NevermindExtension } from '../resources/nevermind-extension-api';
import {
  backgroundJobFixtureAction,
  folderWatchFixtureAction,
  folderWatchView,
} from './ui/background-job-fixtures.ts';
import { chatView } from './ui/chat-fixture.ts';
import { clipboardView } from './ui/clipboard-fixture.ts';
import { crudCollectionView } from './ui/crud-collection-fixture.ts';
import {
  floatingWindowToggleAction,
  floatingWindowView,
} from './ui/floating-window-fixture.ts';
import { formView } from './ui/form-fixture.ts';
import { gridView } from './ui/grid-fixture.ts';
import { emptyListView, listView, loadingView } from './ui/list-fixtures.ts';
import {
  delayedNestedToolView,
  failingNestedTool,
  nestedAiChatView,
  nestedToolWithoutView,
} from './ui/nested-tool-fixtures.ts';
import { ocrFixtureView } from './ui/ocr-fixture.ts';
import { previewView } from './ui/preview-fixture.ts';
import { editorView, promptView } from './ui/prompt-editor-fixtures.ts';
import {
  renderingPolishView,
  sidePreviewView,
} from './ui/rendering-fixtures.ts';
import {
  cameraView,
  progressView,
  webviewView,
} from './ui/simple-view-fixtures.ts';
import { textInputView } from './ui/text-input-fixture.ts';
import { windowManagementSettingsFixtureView } from './ui/window-management-settings-fixture.ts';

const extension: NevermindExtension = {
  id: 'dev.ui-fixtures',
  title: 'Dev UI Fixtures',
  subtitle: 'Dev-only extension API fixtures',
  capabilities: [
    'camera',
    'desktop.files',
    'desktop.apps',
    'clipboard.history',
    'ocr',
  ],
  actions(ctx) {
    return [
      floatingWindowToggleAction(ctx),
      backgroundJobFixtureAction(ctx),
      folderWatchFixtureAction(ctx),
    ];
  },
  commands: [
    {
      id: 'list',
      title: 'Dev UI: List',
      icon: 'list',
      run: (ctx) => listView(ctx),
    },
    {
      id: 'loading',
      title: 'Dev UI: Loading',
      icon: 'loader',
      run: (ctx) => loadingView(ctx),
    },
    {
      id: 'empty-list',
      title: 'Dev UI: Empty List',
      icon: 'list-x',
      run: (ctx) => emptyListView(ctx),
    },
    {
      id: 'crud-collection',
      title: 'Dev UI: CRUD Collection',
      icon: 'list-plus',
      run: (ctx) => crudCollectionView(ctx),
    },
    {
      id: 'rendering-polish',
      title: 'Dev UI: Rendering Polish',
      icon: 'sparkles',
      run: (ctx) => renderingPolishView(ctx),
    },
    {
      id: 'side-preview',
      title: 'Dev UI: Side Preview',
      icon: 'panel-right',
      run: (ctx) => sidePreviewView(ctx),
    },
    {
      id: 'grid',
      title: 'Dev UI: Grid',
      icon: 'grid',
      run: (ctx) => gridView(ctx),
    },
    {
      id: 'ocr',
      title: 'Dev UI: OCR',
      icon: 'scan-text',
      run: (ctx) => ocrFixtureView(ctx),
    },
    {
      id: 'preview',
      title: 'Dev UI: Preview',
      icon: 'file-text',
      run: (ctx) => previewView(ctx),
    },
    {
      id: 'form',
      title: 'Dev UI: Form',
      icon: 'list-checks',
      run: (ctx) => formView(ctx),
    },
    {
      id: 'text-input',
      title: 'Dev UI: Text Input',
      icon: 'keyboard',
      run: (ctx) => textInputView(ctx),
    },
    {
      id: 'window-management-settings',
      title: 'Dev UI: Window Management Settings',
      icon: 'ruler',
      run: (ctx) => windowManagementSettingsFixtureView(ctx),
    },
    {
      id: 'clipboard',
      title: 'Dev UI: Clipboard',
      icon: 'clipboard',
      run: (ctx) => clipboardView(ctx),
    },
    {
      id: 'folder-watch',
      title: 'Dev UI: Folder Watch',
      icon: 'folder-sync',
      run: (ctx) => folderWatchView(ctx),
    },
    {
      id: 'floating-window',
      title: 'Dev UI: Floating Window',
      icon: 'panel-top-open',
      run: (ctx) => floatingWindowView(ctx),
    },
    {
      id: 'prompt',
      title: 'Dev UI: Prompt',
      icon: 'text-cursor-input',
      run: (ctx) => promptView(ctx),
    },
    {
      id: 'editor',
      title: 'Dev UI: Editor',
      icon: 'file-pen-line',
      run: (ctx) => editorView(ctx),
    },
    {
      id: 'chat',
      title: 'Dev UI: Chat',
      icon: 'message-circle',
      run: (ctx) => chatView(ctx),
    },
    {
      id: 'nested-ai-chat',
      title: 'Dev UI: Nested AI Chat Lifecycle',
      icon: 'message-circle',
      run: (ctx) => nestedAiChatView(ctx),
    },
    {
      id: 'nested-delayed-tool',
      title: 'Dev UI: Delayed Nested Tool',
      icon: 'timer',
      run: (ctx) => delayedNestedToolView(ctx),
    },
    {
      id: 'nested-empty-tool',
      title: 'Dev UI: Nested Tool Without a View',
      icon: 'timer',
      run: nestedToolWithoutView,
    },
    {
      id: 'nested-error-tool',
      title: 'Dev UI: Nested Tool Failure',
      icon: 'triangle-alert',
      run: failingNestedTool,
    },
    {
      id: 'progress',
      title: 'Dev UI: Progress',
      icon: 'loader',
      run: (ctx) => progressView(ctx),
    },
    {
      id: 'webview',
      title: 'Dev UI: Webview',
      icon: 'globe',
      run: (ctx) => webviewView(ctx),
    },
    {
      id: 'camera',
      title: 'Dev UI: Camera',
      icon: 'camera',
      run: (ctx) => cameraView(ctx),
    },
  ],
};

export default extension;
