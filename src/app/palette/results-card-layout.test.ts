import assert from 'node:assert/strict';
import test from 'node:test';
import { resultsCardClassName } from './results-card-layout';

test('an obscured extension cannot impose its card layout on a confirmation', () => {
  const view = {
    type: 'form',
    contentSizing: 'fit',
    size: 'large',
    presentation: 'side-preview',
    aiChat: {},
    isLoading: true,
  };
  assert.equal(
    resultsCardClassName({
      stacked: false,
      obscured: true,
      confirmation: true,
      builder: true,
      view,
    }),
    'results card resultsCard fitContentCard',
  );
  assert.equal(
    resultsCardClassName({
      stacked: false,
      obscured: true,
      confirmation: false,
      builder: true,
      view,
    }),
    'results card resultsCard',
  );
  assert.equal(
    resultsCardClassName({
      stacked: true,
      obscured: false,
      confirmation: false,
      builder: true,
      view,
    }),
    'results card optionsCard formResultsCard fitContentCard largeResultsCard aiChatResultsCard builderResultsCard sidePreviewCard loadingBorder',
  );
});
