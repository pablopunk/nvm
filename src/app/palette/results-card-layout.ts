interface ResultsCardLayout {
  stacked: boolean;
  obscured: boolean;
  confirmation: boolean;
  builder: boolean;
  view?: {
    type?: string;
    contentSizing?: string;
    size?: string;
    presentation?: string;
    aiChat?: unknown;
    isLoading?: boolean;
  } | null;
}

export function resultsCardClassName({
  stacked,
  obscured,
  confirmation,
  builder,
  view,
}: ResultsCardLayout): string {
  const visible = !obscured && view;
  return [
    'results',
    'card',
    stacked ? 'optionsCard' : 'resultsCard',
    visible && view.type === 'form' && 'formResultsCard',
    (confirmation || (visible && view.contentSizing === 'fit')) &&
      'fitContentCard',
    visible && view.size === 'large' && 'largeResultsCard',
    visible && view.aiChat && 'aiChatResultsCard',
    visible && builder && 'builderResultsCard',
    visible && view.presentation === 'side-preview' && 'sidePreviewCard',
    visible && view.isLoading && 'loadingBorder',
  ]
    .filter(Boolean)
    .join(' ');
}
