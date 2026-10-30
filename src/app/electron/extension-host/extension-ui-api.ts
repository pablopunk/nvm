// biome-ignore-all lint: Host-side extension payload boundary keeps dynamic view shapes by design.
import { feedbackView } from '../../palette/feedback';
import { extensionLoadingView } from '../../palette/model';
import {
  createExtensionUiPreview,
  type ExtensionUiPreviewDeps,
} from './extension-ui-preview';

export type ExtensionUiApiDeps = ExtensionUiPreviewDeps & {
  progressView: (input?: unknown) => unknown;
  buildConfirmAction: (input?: unknown) => unknown;
  showIndicator?: (input: unknown) => void;
  updateIndicator?: (input: unknown) => void;
  hideIndicator?: (id?: string) => void;
};

const RESERVED_ITEM_ID_PREFIX = '__nvm:';

function collectionRecord(item: any) {
  const { preview, edit, remove, ...record } = item;
  if (
    typeof record.id === 'string' &&
    record.id.startsWith(RESERVED_ITEM_ID_PREFIX)
  )
    throw new Error(
      `Collection item ids must not start with "${RESERVED_ITEM_ID_PREFIX}"`,
    );
  const editAction = edit
    ? { shortcut: 'Command+E', ...edit, title: edit.title || 'Edit' }
    : null;
  const removeAction = remove
    ? {
        shortcut: 'Command+Backspace',
        style: 'destructive',
        requiresConfirmation: true,
        confirmMessage: `Remove “${record.title || 'item'}”?`,
        confirmLabel: 'Remove',
        ...remove,
        title: remove.title || 'Remove',
      }
    : null;
  const actions = [preview, editAction, removeAction].filter(Boolean);
  return {
    ...record,
    primaryAction: preview || editAction || removeAction,
    actions,
    actionPanel: actions.length ? { sections: [{ actions }] } : undefined,
  };
}

function collectionCreateItem(add: any) {
  return {
    id: `${RESERVED_ITEM_ID_PREFIX}collection-create`,
    title: add.title || 'Create item',
    subtitle: add.subtitle,
    icon: add.icon || 'plus',
    primaryAction: add,
    actions: [add],
    actionPanel: { sections: [{ actions: [add] }] },
  };
}

export function createExtensionUiApi({
  buildPreviewItemAction,
  readTextFilePreview,
  progressView,
  buildConfirmAction,
  showIndicator = () => {},
  updateIndicator = () => {},
  hideIndicator = () => {},
}: ExtensionUiApiDeps) {
  const { previewView, previewFile, previewAction, legacyPreview } =
    createExtensionUiPreview({ buildPreviewItemAction, readTextFilePreview });
  return {
    list: (view: any) => ({ ...view, type: 'list' }),
    collection: (input: any = {}) => {
      const add = input.add;
      const records = Array.isArray(input.items)
        ? input.items.map(collectionRecord)
        : [];
      return {
        id: input.id,
        type: 'list',
        title: input.title || 'Collection',
        subtitle: input.subtitle,
        searchBarPlaceholder: input.searchBarPlaceholder,
        emptyView: input.emptyView,
        windowPresentation: input.windowPresentation,
        actions: add ? [add] : [],
        actionPanel: add ? { sections: [{ actions: [add] }] } : undefined,
        items: add ? [collectionCreateItem(add), ...records] : records,
      };
    },
    grid: (view: any) => ({ ...view, type: 'grid' }),
    preview: legacyPreview,
    previewView,
    previewFile,
    previewAction,
    chat: (view: any) => ({ ...view, type: 'chat' }),
    form: (view: any) => ({ ...view, type: 'form' }),
    editor: (view: any) => ({ ...view, type: 'editor' }),
    progress: (input: any = {}) => progressView(input),
    confirm: (input: any = {}) => buildConfirmAction(input),
    toast: (input: any = {}) => {
      const tone = String(input?.tone || 'default');
      showIndicator({
        id: 'feedback',
        title: 'Nevermind',
        subtitle: String(input?.message || ''),
        ...(tone === 'error' || tone === 'success' ? { status: tone } : {}),
        durationMs: tone === 'error' ? 4000 : 2200,
      });
    },
    indicator: {
      show: showIndicator,
      update: updateIndicator,
      hide: hideIndicator,
    },
    webview: (view: any) => ({ ...view, type: 'webview' }),
    camera: (view = {}) => ({
      title: 'Camera',
      size: 'large',
      muted: true,
      ...view,
      type: 'camera',
    }),
    item: (item: any) => item,
    actions: (actions: any) => actions,
    empty: (title = 'Nothing here', subtitle = '') => ({
      type: 'preview',
      title,
      content: `# ${title}${subtitle ? `\n\n${subtitle}` : ''}`,
    }),
    loading: (title = 'Loading…') => extensionLoadingView(title),
    error: (title = 'Something went wrong', message = '') =>
      feedbackView({
        id: 'extension-error',
        title,
        message: message || 'Try again.',
        tone: 'error',
      }),
  };
}
