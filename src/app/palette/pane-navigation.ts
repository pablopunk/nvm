import type { CommandFormValue, CommandView } from './model';

export type PaneView = {
  id: string;
  view: CommandView;
  query: string;
  selection: string;
  formValues: Record<string, CommandFormValue>;
  operationId?: string;
};
export type PaneFrame = {
  id: string;
  requestId?: string;
  generation: number;
  primary: boolean;
  history: PaneView[];
};
export type PaneState = { frames: PaneFrame[] };
export type PaneOwner = {
  frameId: string;
  viewId: string;
  generation: number;
  operationId?: string;
};

export function activePane(state: PaneState) {
  return state.frames.at(-1);
}
export function paneView(frame?: PaneFrame) {
  return frame?.history.at(-1);
}
export function createPaneView(view: CommandView, id: string): PaneView {
  const formValues: PaneView['formValues'] = {};
  for (const field of view.fields || []) {
    if (field.value !== undefined) formValues[field.id] = field.value;
  }
  return {
    id,
    view,
    query: '',
    selection: view.selectedItemId || '',
    formValues,
  };
}
export function paneOwner(state: PaneState): PaneOwner | null {
  const frame = activePane(state);
  const view = paneView(frame);
  return frame && view
    ? {
        frameId: frame.id,
        viewId: view.id,
        generation: frame.generation,
        operationId: view.operationId,
      }
    : null;
}
export function ownerIsCurrent(state: PaneState, owner: PaneOwner | null) {
  if (!owner) return state.frames.length === 0;
  const current = paneOwner(state);
  return (
    current?.frameId === owner.frameId &&
    current.viewId === owner.viewId &&
    current.generation === owner.generation &&
    (!owner.operationId || current.operationId === owner.operationId)
  );
}
export function ownerIsRetained(state: PaneState, owner: PaneOwner) {
  const frame = state.frames.find(
    (candidate) => candidate.id === owner.frameId,
  );
  const entry = paneView(frame);
  return (
    frame?.generation === owner.generation &&
    entry?.id === owner.viewId &&
    (!owner.operationId || entry.operationId === owner.operationId)
  );
}

/** Hiding cancels transient work while retaining the last live chat. */
export function hidePanes(state: PaneState): PaneState {
  for (const frame of [...state.frames].reverse()) {
    const index = frame.history.findLastIndex((entry) => entry.view.aiChat);
    if (index < 0) continue;
    return {
      frames: [
        {
          ...frame,
          primary: false,
          requestId: undefined,
          generation: frame.generation + 1,
          history: frame.history
            .slice(0, index + 1)
            .map((entry) => ({ ...entry, operationId: undefined })),
        },
      ],
    };
  }
  return { frames: [] };
}
export function openPane(
  state: PaneState,
  view: CommandView,
  id: string,
  options: { requestId?: string; primary?: boolean; reset?: boolean } = {},
): PaneState {
  const frame: PaneFrame = {
    id,
    requestId: options.requestId,
    primary: Boolean(options.primary),
    generation: 0,
    history: [createPaneView(view, `${id}:view`)],
  };
  return { frames: [...(options.reset ? [] : state.frames), frame] };
}
export function resolvePane(
  state: PaneState,
  requestId: string,
  view: CommandView | undefined,
): PaneState {
  const frame = state.frames.find(
    (candidate) => candidate.requestId === requestId,
  );
  if (!frame) return state;
  if (!view)
    return { frames: state.frames.filter((candidate) => candidate !== frame) };
  return {
    frames: state.frames.map((candidate) =>
      candidate === frame
        ? {
            ...frame,
            requestId: undefined,
            history: [createPaneView(view, paneView(frame)!.id)],
          }
        : candidate,
    ),
  };
}
export function dismissPane(state: PaneState, owner: PaneOwner): PaneState {
  if (!ownerIsCurrent(state, owner)) return state;
  return { frames: state.frames.slice(0, -1) };
}
export function navigatePane(
  state: PaneState,
  view: CommandView,
  mode: 'root' | 'push' | 'replace',
  id: string,
): PaneState {
  const frame = activePane(state);
  if (!frame) return openPane(state, view, id);
  const entry = createPaneView(view, id);
  const history =
    mode === 'root'
      ? [entry]
      : mode === 'push'
        ? [...frame.history, entry]
        : [...frame.history.slice(0, -1), entry];
  return {
    frames: [
      ...state.frames.slice(0, -1),
      {
        ...frame,
        requestId: undefined,
        generation: frame.generation + 1,
        history,
      },
    ],
  };
}
export function popPane(state: PaneState): PaneState {
  const frame = activePane(state);
  if (!frame) return state;
  if (frame.history.length === 1) return { frames: state.frames.slice(0, -1) };
  return {
    frames: [
      ...state.frames.slice(0, -1),
      {
        ...frame,
        requestId: undefined,
        generation: frame.generation + 1,
        history: frame.history.slice(0, -1),
      },
    ],
  };
}
