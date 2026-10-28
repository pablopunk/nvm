// biome-ignore-all lint/suspicious/noExplicitAny: fake extension contexts mirror the untyped runtime bridge.
import assert from 'node:assert/strict';
import test from 'node:test';
import { windowManagementSettingsAction } from './window-management-settings-action';
import { fakeWindowManagementContext } from './window-management-test-doubles';

test('the Window Management Settings action pushes the settings list', async () => {
  const { ctx } = fakeWindowManagementContext();
  const action = windowManagementSettingsAction(ctx) as any;

  const result = await action.handler(ctx);

  assert.equal(action.title, 'Window Management Settings');
  assert.equal(action.icon, 'settings');
  assert.equal(result.navigation, 'push');
  assert.equal(result.view.type, 'list');
  assert.equal(result.view.id, 'window-management-settings');
});
