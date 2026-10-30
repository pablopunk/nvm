import { actionsFromPanel, type CommandActionPanel } from './model';

export function rootSearchQueryAction(
  action: { actionPanel?: CommandActionPanel } | null | undefined,
  title: 'Continue Calculation' | 'Swap Units',
) {
  return actionsFromPanel(action?.actionPanel, []).find(
    (candidate) =>
      candidate.type === 'setSearchQuery' && candidate.title === title,
  );
}

export function rootPreviewAction(action: {
  actionPanel?: CommandActionPanel;
}) {
  return actionsFromPanel(action.actionPanel).find(
    (candidate) =>
      candidate.previewIntent || candidate.type === 'previewClipboardItem',
  );
}

export function previewActionFlags(panel?: CommandActionPanel) {
  const actions = actionsFromPanel(panel);
  return {
    hasPreview: actions.some(
      (action) =>
        action.previewIntent || action.type === 'previewClipboardItem',
    ),
    hasQuickLook: actions.some((action) => action.type === 'quickLook'),
  };
}
