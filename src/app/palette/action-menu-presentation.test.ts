import assert from 'node:assert/strict';
import test from 'node:test';
import {
  actionMenuHostIsStacked,
  actionMenuPresentation,
  activePaneIsObscured,
  confirmationReturnSurface,
} from './action-menu-presentation';

test('action menus and nested submenus are always compact', () => {
  assert.equal(actionMenuPresentation('actions'), 'compact');
  assert.equal(actionMenuPresentation('submenu'), 'compact');
});

test('confirmations stay compact while prompts use the full palette', () => {
  assert.equal(actionMenuPresentation('confirmation'), 'compact');
  assert.equal(actionMenuPresentation('prompt'), 'default');
});

test('confirmation cancellation returns to its exact source surface', () => {
  assert.equal(confirmationReturnSurface(true, false), 'submenu');
  assert.equal(confirmationReturnSurface(false, true), 'panel');
  assert.equal(confirmationReturnSurface(false, false), 'view');
});

test('compact action menus preserve their host view geometry', () => {
  assert.equal(
    actionMenuHostIsStacked({
      builderWorkspaceVisible: false,
      hasNavigatedChild: true,
      hasSiblingViews: false,
      isRootLikeView: false,
    }),
    true,
  );
  assert.equal(
    actionMenuHostIsStacked({
      builderWorkspaceVisible: false,
      hasNavigatedChild: false,
      hasSiblingViews: false,
      isRootLikeView: false,
    }),
    false,
  );
  assert.equal(
    actionMenuHostIsStacked({
      builderWorkspaceVisible: false,
      hasNavigatedChild: true,
      hasSiblingViews: false,
      isRootLikeView: true,
    }),
    false,
  );
  assert.equal(
    actionMenuHostIsStacked({
      builderWorkspaceVisible: true,
      hasNavigatedChild: true,
      hasSiblingViews: true,
      isRootLikeView: false,
    }),
    true,
  );
});

test('compact action menus float over the active pane instead of hiding it', () => {
  assert.equal(activePaneIsObscured({ itemActions: true }), false);
  assert.equal(activePaneIsObscured({ rootActions: true }), false);
  assert.equal(
    activePaneIsObscured({ actionSubmenu: true, shortcutOptions: true }),
    false,
  );
  assert.equal(activePaneIsObscured({}), false);
});

test('surfaces rendered in the results card still replace the active pane', () => {
  assert.equal(activePaneIsObscured({ confirmation: true }), true);
  assert.equal(activePaneIsObscured({ prompt: true }), true);
  assert.equal(
    activePaneIsObscured({ itemActions: true, alias: { id: 'a' } }),
    true,
  );
  assert.equal(activePaneIsObscured({ preview: { title: 'Photo' } }), true);
});
