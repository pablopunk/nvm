import { type SetStateAction, useLayoutEffect, useRef, useState } from 'react';
import type { CommandView } from './model';
import {
  activePane,
  createPaneView,
  dismissPane,
  hidePanes,
  navigatePane,
  openPane,
  ownerIsCurrent,
  ownerIsRetained,
  paneOwner,
  paneView,
  popPane,
  resolvePane,
  type PaneOwner,
  type PaneState,
  type PaneView,
} from './pane-navigation';

export type NavigationMode = 'root' | 'push' | 'replace' | 'pop';
export type NavigationState = {
  view: CommandView | null;
  backStack: CommandView[];
};
export function nextNavigationState(
  current: NavigationState,
  nextView: CommandView,
  navigation: Exclude<NavigationMode, 'pop'> = 'replace',
): NavigationState {
  if (navigation === 'root') return { view: nextView, backStack: [] };
  if (navigation === 'push' && current.view)
    return { view: nextView, backStack: [...current.backStack, current.view] };
  return { view: nextView, backStack: current.backStack };
}
export function previousNavigationState(current: NavigationState) {
  const backStack = [...current.backStack];
  const view = backStack.pop() || null;
  return { state: { view, backStack }, didPop: Boolean(view) };
}
function applyValue<T>(value: SetStateAction<T>, current: T): T {
  return typeof value === 'function'
    ? (value as (current: T) => T)(current)
    : value;
}
export function useExtensionNavigation(focusSearch?: () => void) {
  const [state, setState] = useState<PaneState>({ frames: [] });
  const stateRef = useRef(state);
  const [navigationKey, setNavigationKey] = useState(0);
  const focusOrigins = useRef(
    new Map<
      string,
      {
        element: HTMLElement;
        start?: number | null;
        end?: number | null;
      }
    >(),
  );
  function update(
    transition: (state: PaneState) => PaneState,
    navigation = false,
  ) {
    const next = transition(stateRef.current);
    if (next === stateRef.current) return;
    if (navigation) {
      const entry = paneView(activePane(stateRef.current));
      const element = document.activeElement;
      if (entry && element instanceof HTMLElement) {
        const selectable =
          element instanceof HTMLTextAreaElement ||
          element instanceof HTMLInputElement;
        focusOrigins.current.set(entry.id, {
          element,
          start: selectable ? element.selectionStart : undefined,
          end: selectable ? element.selectionEnd : undefined,
        });
      }
    }
    stateRef.current = next;
    setState(next);
    if (navigation) setNavigationKey((key) => key + 1);
  }
  function updateEntry(transition: (entry: PaneView) => PaneView) {
    update((current) => {
      const frame = activePane(current);
      const entry = paneView(frame);
      if (!frame || !entry) return current;
      return {
        frames: [
          ...current.frames.slice(0, -1),
          {
            ...frame,
            history: [...frame.history.slice(0, -1), transition(entry)],
          },
        ],
      };
    });
  }
  function showView(
    view: CommandView,
    mode: Exclude<NavigationMode, 'pop'> = 'replace',
  ) {
    update(
      (current) => navigatePane(current, view, mode, crypto.randomUUID()),
      true,
    );
  }
  function setView(value: SetStateAction<CommandView | null>) {
    updateEntry((entry) => ({
      ...entry,
      view: applyValue(value, entry.view) || entry.view,
    }));
  }
  function setBackStack(value: SetStateAction<CommandView[]>) {
    update((current) => {
      const frame = activePane(current);
      const entry = paneView(frame);
      if (!frame || !entry) return current;
      const views = applyValue(
        value,
        frame.history.slice(0, -1).map((item) => item.view),
      );
      return {
        frames: [
          ...current.frames.slice(0, -1),
          {
            ...frame,
            history: [
              ...views.map((view, index) =>
                frame.history[index]?.view === view
                  ? frame.history[index]
                  : createPaneView(view, crypto.randomUUID()),
              ),
              entry,
            ],
          },
        ],
      };
    });
  }
  function clearView() {
    update(() => ({ frames: [] }), true);
  }
  function clearSiblings() {
    update((current) => ({ frames: current.frames.slice(-1) }));
  }
  function popView() {
    const hadParent =
      stateRef.current.frames.length > 1 ||
      (activePane(stateRef.current)?.history.length || 0) > 1;
    update(popPane, true);
    return hadParent;
  }
  function startShortcut(
    view: CommandView,
    requestId: string,
    primary: boolean,
  ) {
    update(
      (current) =>
        openPane(current, view, requestId, {
          requestId,
          primary,
          reset: primary,
        }),
      true,
    );
  }
  function resolveShortcut(requestId: string, view?: CommandView) {
    const exists = stateRef.current.frames.some(
      (frame) => frame.requestId === requestId,
    );
    update((current) => resolvePane(current, requestId, view), true);
    return exists;
  }
  function beginOperation() {
    updateEntry((entry) => ({ ...entry, operationId: crypto.randomUUID() }));
    return paneOwner(stateRef.current);
  }
  function updateOwnedView(
    owner: PaneOwner,
    transition: (view: CommandView) => CommandView,
  ) {
    update((current) => {
      if (!ownerIsRetained(current, owner)) return current;
      return {
        frames: current.frames.map((frame) =>
          frame.id !== owner.frameId
            ? frame
            : {
                ...frame,
                history: frame.history.map((entry) =>
                  entry.id !== owner.viewId
                    ? entry
                    : { ...entry, view: transition(entry.view) },
                ),
              },
        ),
      };
    });
  }
  function updateViewById(
    viewId: string | undefined,
    transition: (view: CommandView) => CommandView,
  ) {
    update((current) => {
      const entries = current.frames.flatMap((frame) => frame.history);
      const matching = viewId
        ? entries.filter((entry) => entry.view.id === viewId)
        : [paneView(activePane(current))].filter((entry): entry is PaneView =>
            Boolean(entry),
          );
      if (matching.length !== 1) return current;
      const target = matching[0];
      return {
        frames: current.frames.map((frame) => ({
          ...frame,
          history: frame.history.map((entry) =>
            entry === target
              ? { ...entry, view: transition(entry.view) }
              : entry,
          ),
        })),
      };
    });
  }
  function dismiss(owner: PaneOwner) {
    update((current) => dismissPane(current, owner), true);
  }
  const frame = activePane(state);
  const entry = paneView(frame);
  useLayoutEffect(() => {
    const origin = entry && focusOrigins.current.get(entry.id);
    if (!origin) {
      if (entry && !['chat', 'editor'].includes(entry.view.type)) {
        const animation = requestAnimationFrame(() => {
          if (paneView(activePane(stateRef.current))?.id === entry.id)
            focusSearch?.();
        });
        return () => cancelAnimationFrame(animation);
      }
      return;
    }
    const animation = requestAnimationFrame(() => {
      if (
        !origin.element.isConnected ||
        origin.element.closest('[inert], [hidden]')
      )
        return;
      origin.element.focus();
      if (
        (origin.element instanceof HTMLInputElement ||
          origin.element instanceof HTMLTextAreaElement) &&
        origin.start != null &&
        origin.end != null
      )
        origin.element.setSelectionRange(origin.start, origin.end);
    });
    return () => cancelAnimationFrame(animation);
  }, [entry?.id]);
  return {
    view: entry?.view || null,
    backStack: frame?.history.slice(0, -1).map((item) => item.view) || [],
    frames: state.frames,
    activeFrame: frame,
    activeEntry: entry,
    siblingViews: state.frames
      .slice(0, -1)
      .flatMap((item) => paneView(item)?.view || []),
    navigationKey,
    showView,
    setView,
    setBackStack,
    clearView,
    clearSiblings,
    popView,
    startShortcut,
    resolveShortcut,
    beginOperation,
    updateOwnedView,
    updateViewById,
    hide: () => update(hidePanes, true),
    isRetained: (owner: PaneOwner) => ownerIsRetained(stateRef.current, owner),
    dismiss,
    capture: () => paneOwner(stateRef.current),
    isCurrent: (owner: PaneOwner | null) =>
      ownerIsCurrent(stateRef.current, owner),
    currentView: () => paneView(activePane(stateRef.current))?.view || null,
    currentFrame: () => activePane(stateRef.current),
    setQuery: (value: SetStateAction<string>) =>
      updateEntry((entry) => ({
        ...entry,
        query: applyValue(value, entry.query),
      })),
    setSelection: (selection: string) =>
      updateEntry((entry) => ({ ...entry, selection })),
    setFormValues: (value: SetStateAction<PaneView['formValues']>) =>
      updateEntry((entry) => ({
        ...entry,
        formValues: applyValue(value, entry.formValues),
      })),
  };
}
