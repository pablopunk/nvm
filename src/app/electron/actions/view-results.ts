import { isView } from './view-types';

export interface ViewResultNormalizerDeps {
  normalizeView: (view: any, entry: any) => any;
  normalizeViewItems: (items: any, entry: any) => any;
}

export function createViewResultNormalizer(deps: ViewResultNormalizerDeps) {
  const { normalizeView, normalizeViewItems } = deps;

  function normalizeExtensionView(result, entry) {
    if (!result) return null;
    const view = isView(result)
      ? result
      : isView(result.view)
        ? result.view
        : null;
    return view ? normalizeView(view, entry) : null;
  }

  function normalizeViewPatch(patch, entry) {
    if (!patch) return patch;
    return {
      ...patch,
      items: normalizeViewItems(patch.items, entry),
    };
  }

  function normalizeHostViewResult(result) {
    if (!result) return result;
    return {
      ...result,
      ...(result.view ? { view: normalizeView(result.view, null) } : {}),
      ...(result.patch
        ? { patch: normalizeViewPatch(result.patch, null) }
        : {}),
    };
  }

  return {
    normalizeExtensionView,
    normalizeViewPatch,
    normalizeHostViewResult,
  };
}
