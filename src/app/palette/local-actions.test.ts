import assert from 'node:assert/strict';
import test from 'node:test';
import {
  combinedActionSections,
  findLocalShortcut,
  localActionCandidates,
} from './local-actions';
import type { CommandAction, CommandItem, CommandView } from './model';

const primary: CommandAction = {
  type: 'copyText',
  title: 'Item',
  shortcut: 'Control+N',
};
const viewAction: CommandAction = {
  type: 'copyText',
  title: 'Next page',
  shortcut: 'Control+N',
};
const item: CommandItem = {
  id: 'item',
  title: 'Item',
  primaryAction: primary,
  actions: [],
};
const view: CommandView = {
  type: 'grid',
  title: 'Grid',
  actions: [viewAction],
};

test('selected item shortcuts take precedence while view actions remain available', () => {
  assert.equal(
    findLocalShortcut(localActionCandidates(item, view), 'Control+N'),
    primary,
  );
  assert.equal(
    findLocalShortcut(localActionCandidates(null, view), 'Control+N'),
    viewAction,
  );
  assert.deepEqual(combinedActionSections(item, view).at(-1)?.actions, [
    viewAction,
  ]);
});

test('view actions are deduplicated and hidden panels omit their view section', () => {
  assert.equal(
    combinedActionSections(item, { ...view, actions: [primary] }).length,
    1,
  );
  assert.equal(
    combinedActionSections(item, { ...view, actionPanelVisibility: 'hidden' })
      .length,
    1,
  );
});
