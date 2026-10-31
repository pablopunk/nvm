import assert from 'node:assert/strict';
import test from 'node:test';
import {
  combinedActionSections,
  findLocalShortcut,
  firstViewActionRowValue,
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

test('selects the first inline view action row, skipping empty panel sections', () => {
  const next = {
    type: 'runExtensionAction',
    title: 'Continue',
  } as CommandAction;
  assert.equal(
    firstViewActionRowValue({
      title: 'Wizard',
      actions: [next],
    } as CommandView),
    'extension-view:0:0:runExtensionAction:Continue',
  );
  assert.equal(
    firstViewActionRowValue({
      title: 'Wizard',
      actionPanel: {
        sections: [{ title: 'Empty', actions: [] }, { actions: [next] }],
      },
    } as CommandView),
    'extension-view:1:0:runExtensionAction:Continue',
  );
  assert.equal(firstViewActionRowValue({ title: 'Plain' } as CommandView), '');
});
