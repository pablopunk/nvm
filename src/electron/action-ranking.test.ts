import assert from 'node:assert/strict';
import test from 'node:test';
import { compareRankedActions } from './action-ranking';
import { scoreNormalized } from './search-utils';

const EXACT_MATCH_SCORE = 100;

test('generated actions sort ahead of their AI builder chat on ranking ties', () => {
  const generatedAction = {
    id: 'extension:generated.quit-all-apps:quit-all-apps',
    kind: 'extension-action',
    title: 'Quit All Apps',
    score: EXACT_MATCH_SCORE,
    lastUsed: 0,
  };
  const builderChat = {
    id: 'extension-root:nevermind.ai-builder:ai-chat:quit-all-apps',
    kind: 'extension-root-item',
    extensionId: 'nevermind.ai-builder',
    title: 'Continue AI chat: quit all apps',
    score: EXACT_MATCH_SCORE,
    lastUsed: Date.now(),
  };

  assert.deepEqual(
    [builderChat, generatedAction]
      .sort(compareRankedActions)
      .map((item) => item.kind),
    ['extension-action', 'extension-root-item'],
  );
});

test('extension actions do not get a global tie priority over other result kinds', () => {
  const appResult = {
    id: 'app:calculator',
    kind: 'app',
    title: 'Calculator',
    score: EXACT_MATCH_SCORE,
    lastUsed: Date.now(),
  };
  const extensionAction = {
    id: 'extension:calculator:calculator',
    kind: 'extension-action',
    title: 'Calculator',
    score: EXACT_MATCH_SCORE,
    lastUsed: 0,
  };

  assert.deepEqual(
    [extensionAction, appResult]
      .sort(compareRankedActions)
      .map((item) => item.kind),
    ['app', 'extension-action'],
  );
});

test('AI builder chat continuation title no longer exact-matches the prompt', () => {
  const query = 'quit all apps';

  assert.equal(scoreNormalized('Quit All Apps', query), EXACT_MATCH_SCORE);
  assert.ok(
    scoreNormalized('Continue AI chat: quit all apps', query) <
      EXACT_MATCH_SCORE,
  );
});
