import { feedbackView } from './feedback';
import { allViewItems } from './filtering';
import type {
  BuilderPreview,
  CommandAction,
  CommandView,
  CommandViewPatch,
} from './model';
import { patchCommandView } from './view-patches';

export function builderPreviewRootActions(preview: BuilderPreview) {
  return [...preview.preview.rootItems, ...preview.preview.actions];
}

export function builderPreviewShouldAutoRun(preview: BuilderPreview) {
  return builderPreviewRootActions(preview).length > 0;
}

export function builderPreviewAutoRunAction(preview: BuilderPreview) {
  return preview.preview.actions[0] || preview.preview.rootItems[0];
}

export function builderPreviewResultIsCurrent(
  actionVersion: number,
  currentVersion: number,
) {
  return actionVersion === currentVersion;
}

export function upsertBuilderPreview<T extends { filename: string }>(
  previews: T[],
  preview: T,
) {
  return [
    ...previews.filter((item) => item.filename !== preview.filename),
    preview,
  ];
}

export function resetBuilderPreviewState<
  T extends {
    filename: string;
    rootView: T['view'];
    view: unknown;
    backStack: unknown[];
  },
>(previews: T[], filename: string) {
  return previews.map((preview) =>
    preview.filename === filename
      ? { ...preview, view: preview.rootView, backStack: [] }
      : preview,
  );
}

export function builderPreviewSelectedItemId(view: CommandView, current = '') {
  const items = allViewItems(view);
  return (
    (view.selectedItemId &&
    items.some((item) => item.id === view.selectedItemId)
      ? view.selectedItemId
      : '') ||
    (current && items.some((item) => item.id === current) ? current : '') ||
    items[0]?.id ||
    ''
  );
}

export function patchBuilderPreviewState<
  T extends { view: CommandView; selectedItemId?: string },
>(preview: T, patch: CommandViewPatch) {
  const view = patchCommandView(preview.view, patch);
  return {
    ...preview,
    view,
    selectedItemId: builderPreviewSelectedItemId(
      view,
      patch.selectedItemId || preview.selectedItemId,
    ),
  };
}

export function patchBuilderPreviewViewById<
  T extends { view: CommandView; selectedItemId?: string },
>(previews: T[], viewId: string, patch: CommandViewPatch) {
  let matched = false;
  const next = previews.map((preview) => {
    if (preview.view.id !== viewId) {
      return preview;
    }
    matched = true;
    return patchBuilderPreviewState(preview, patch);
  });
  return matched ? next : previews;
}

export function replaceBuilderPreviewViewById<
  T extends { view: CommandView; selectedItemId?: string },
>(previews: T[], viewId: string, view: CommandView) {
  let matched = false;
  const next = previews.map((preview) => {
    if (preview.view.id !== viewId) {
      return preview;
    }
    matched = true;
    return {
      ...preview,
      view,
      selectedItemId: builderPreviewSelectedItemId(
        view,
        preview.selectedItemId,
      ),
    };
  });
  return matched ? next : previews;
}

export function retryBuilderPreviewHydration<
  T extends { view: CommandView; selectedItemId?: string },
>(previews: T[], viewId: string) {
  let matched = false;
  const next = previews.map((preview) => {
    const hasRetry = allViewItems(preview.view).some(
      (item) =>
        item.primaryAction?.type === 'nativeAction' &&
        (
          item.primaryAction.nativeAction as {
            kind?: string;
            viewId?: string;
          }
        )?.kind === 'view-hydrate-retry' &&
        (item.primaryAction.nativeAction as { viewId?: string }).viewId ===
          viewId,
    );
    if (!hasRetry) {
      return preview;
    }
    matched = true;
    const view: CommandView = {
      type: 'list',
      id: viewId,
      title: 'Loading...',
      isLoading: true,
      items: [],
    };
    return { ...preview, view, selectedItemId: '' };
  });
  return matched ? next : previews;
}

export function hydrateBuilderPreviewViewById<
  T extends { view: CommandView; selectedItemId?: string },
>(
  previews: T[],
  payload: {
    viewId: string;
    items?: CommandView['items'];
    isLoading?: boolean;
    error?: { message: string };
    retry?: boolean;
  },
) {
  if (payload.error) {
    const retryAction: CommandAction | undefined = payload.retry
      ? {
          type: 'nativeAction',
          title: 'Retry',
          nativeAction: { kind: 'view-hydrate-retry', viewId: payload.viewId },
        }
      : undefined;
    return replaceBuilderPreviewViewById(
      previews,
      payload.viewId,
      feedbackView({
        id: `view-hydrate-error:${payload.viewId}`,
        title: 'Could not load items',
        message: 'Try again or go back.',
        tone: 'error',
        actions: [
          ...(retryAction ? [retryAction] : []),
          { type: 'popView', title: 'Dismiss' },
        ],
      }),
    );
  }
  if (!payload.items) {
    return previews;
  }
  return patchBuilderPreviewViewById(previews, payload.viewId, {
    mode: 'replace',
    items: payload.items,
    isLoading: payload.isLoading === undefined ? false : payload.isLoading,
  });
}

export function applyBuilderPreviewActionResult<
  T extends {
    filename: string;
    rootView: T['view'];
    view: unknown;
    backStack: unknown[];
  },
>(
  previews: T[],
  filename: string,
  result: {
    view?: T['view'];
    navigation?: 'root' | 'push' | 'replace' | 'pop';
  },
) {
  return previews.map((preview) => {
    if (preview.filename !== filename) {
      return preview;
    }
    if (result.navigation === 'pop') {
      const backStack = [...preview.backStack];
      const view = backStack.pop();
      return view ? { ...preview, view, backStack } : preview;
    }
    const view = result.view;
    if (!view) {
      return preview;
    }
    if (result.navigation === 'root') {
      return { ...preview, rootView: view, view, backStack: [] };
    }
    return {
      ...preview,
      view,
      backStack:
        result.navigation === 'replace'
          ? preview.backStack
          : [...preview.backStack, preview.view],
    };
  });
}
