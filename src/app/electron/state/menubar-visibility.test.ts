import assert from 'node:assert/strict';
import test from 'node:test';
import {
  appMenuBarSettingItem,
  createMenuBarVisibility,
} from './menubar-visibility';

function createStore() {
  const state: Record<string, any> = {};
  let saves = 0;
  const visibility = createMenuBarVisibility({
    userStateStore: { get: () => state },
    scheduleSaveState: () => {
      saves += 1;
    },
  });
  return { state, visibility, saves: () => saves };
}

test('menu bar items are shown until the user hides them', () => {
  const { visibility, saves } = createStore();
  assert.equal(visibility.isShown('pomodoro'), true);

  visibility.setShown('pomodoro', false);
  assert.equal(visibility.isShown('pomodoro'), false);

  visibility.setShown('pomodoro', true);
  assert.equal(visibility.isShown('pomodoro'), true);
  assert.equal(saves(), 2);
});

test('the app settings row reads the same state as the palette toggle', () => {
  const { state, visibility } = createStore();
  assert.deepEqual(appMenuBarSettingItem(state).accessories, [{ text: 'On' }]);

  visibility.setShown('nevermind.menubar', false);

  const row = appMenuBarSettingItem(state);
  assert.deepEqual(row.accessories, [{ text: 'Off' }]);
  assert.equal(row.primaryAction.registeredActionId, 'menu-bar-visibility');
  assert.equal(row.primaryAction.extensionId, 'nevermind.menubar');
});
