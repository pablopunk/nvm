import type { ReactNode } from 'react';
import type { CommandView } from '../model';
import type { ExtensionViewRendererProps } from './extension-view';

type InlineViewActionProps = Pick<
  ExtensionViewRendererProps,
  'view' | 'renderActionPanel' | 'actionPanelRows'
>;

function visibleActionPanelRows(view: CommandView, rows: unknown[]) {
  return view.actionPanelVisibility === 'hidden' ||
    view.actionPanelVisibility === 'menu'
    ? []
    : rows;
}

export function inlineViewActions(
  { view, renderActionPanel, actionPanelRows }: InlineViewActionProps,
  prefix = 'extension-view',
) {
  const rows = visibleActionPanelRows(
    view,
    actionPanelRows(view.actionPanel, view.actions || [], prefix, false),
  );
  return rows.length ? renderActionPanel(rows) : null;
}

/** One container so grid-based list hosts cannot stretch the empty state and its actions apart. */
export function listEmptyContent({
  renderEmpty,
  ...actionProps
}: InlineViewActionProps & { renderEmpty: (view: CommandView) => ReactNode }) {
  return (
    <div className="extensionListEmpty">
      {renderEmpty(actionProps.view)}
      {inlineViewActions(actionProps)}
    </div>
  );
}
