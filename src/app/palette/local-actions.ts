import { normalizedShortcut } from './shortcuts';
import {
  actionsFromPanel,
  type CommandAction,
  type CommandItem,
  type CommandView,
} from './model';

export function actionIdentity(action: CommandAction) {
  const nativeId = (action.nativeAction as { id?: string } | undefined)?.id;
  return [
    action.type,
    action.handlerId,
    nativeId,
    action.title,
    action.path,
    action.url,
    action.text,
  ]
    .filter(Boolean)
    .join('\u0000');
}

export function localActionCandidates(
  item: CommandItem | null | undefined,
  view: CommandView | null | undefined,
): CommandAction[] {
  return [
    item?.primaryAction,
    ...actionsFromPanel(item?.actionPanel, item?.actions || []),
    ...actionsFromPanel(view?.actionPanel, view?.actions || []),
  ].filter(Boolean) as CommandAction[];
}

export function viewActionsForItem(
  item: CommandItem | null | undefined,
  view: CommandView | null | undefined,
): CommandAction[] {
  if (view?.actionPanelVisibility === 'hidden') return [];
  const identities = new Set(
    [
      item?.primaryAction,
      ...actionsFromPanel(item?.actionPanel, item?.actions || []),
    ]
      .filter(Boolean)
      .map((action) => actionIdentity(action!)),
  );
  return actionsFromPanel(view?.actionPanel, view?.actions || []).filter(
    (action) => !identities.has(actionIdentity(action)),
  );
}

export function combinedActionSections(
  item: CommandItem | null | undefined,
  view: CommandView | null | undefined,
) {
  const actions = viewActionsForItem(item, view);
  return [
    ...(item?.actionPanel?.sections?.length
      ? item.actionPanel.sections
      : [{ actions: item?.actions || [] }]),
    ...(actions.length ? [{ title: 'View', actions }] : []),
  ];
}

export function findLocalShortcut(
  actions: CommandAction[],
  accelerator: string,
) {
  return actions.find(
    (action) =>
      normalizedShortcut(action.shortcut) === normalizedShortcut(accelerator),
  );
}
