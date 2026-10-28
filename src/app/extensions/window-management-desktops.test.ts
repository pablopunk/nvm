import assert from 'node:assert/strict';
import test from 'node:test';
import {
  DESKTOP_SWITCH_FAILURE_MESSAGE,
  desktopSwitchScript,
  windowDesktopActions,
} from './window-management-desktops';
import { fakeWindowManagementContext } from './window-management-test-doubles';

const SWITCHED = { stdout: '', stderr: '', exitCode: 0 };
const [next, previous] = windowDesktopActions();
const SCRIPT_FAILED = {
  stdout: '',
  stderr: 'System Events got an error: Not authorized.',
  exitCode: 1,
};

test('next desktop sends Control+Right arrow through System Events', () => {
  assert.equal(
    desktopSwitchScript('next'),
    'tell application "System Events" to key code 124 using control down',
  );
});

test('previous desktop sends Control+Left arrow through System Events', () => {
  assert.equal(
    desktopSwitchScript('previous'),
    'tell application "System Events" to key code 123 using control down',
  );
});

test('desktop commands are no-view and titled for switching desktops', () => {
  assert.deepEqual(
    windowDesktopActions().map((action) => [
      action.id,
      action.title,
      action.subtitle,
      action.mode,
    ]),
    [
      ['next-desktop', 'Next Desktop', 'Switch desktop', 'noView'],
      ['previous-desktop', 'Previous Desktop', 'Switch desktop', 'noView'],
    ],
  );
});

test('a successful switch stays silent and runs the matching script', async () => {
  const { ctx, appleScriptCalls, indicators } = fakeWindowManagementContext({
    appleScript: () => Promise.resolve(SWITCHED),
  });

  await next.run(ctx);

  assert.deepEqual(appleScriptCalls, [[desktopSwitchScript('next')]]);
  assert.equal(indicators.length, 0);
});

test('a failed switch explains the Mission Control and Automation requirements', async () => {
  const { ctx, indicators, logs } = fakeWindowManagementContext({
    appleScript: () => Promise.resolve(SCRIPT_FAILED),
  });

  await previous.run(ctx);

  assert.equal(indicators.length, 1);
  assert.equal(indicators[0].title, 'Previous Desktop');
  assert.equal(indicators[0].subtitle, DESKTOP_SWITCH_FAILURE_MESSAGE);
  assert.equal(indicators[0].status, 'error');
  assert.ok(DESKTOP_SWITCH_FAILURE_MESSAGE.includes('Move left a space'));
  assert.ok(DESKTOP_SWITCH_FAILURE_MESSAGE.includes('Automation'));
  assert.equal(logs[0]?.level, 'error');
});

test('a missing shell facet is reported as the same failure', async () => {
  const { ctx, indicators } = fakeWindowManagementContext();
  ctx.desktop.shell = undefined;

  await next.run(ctx);

  assert.equal(indicators[0]?.subtitle, DESKTOP_SWITCH_FAILURE_MESSAGE);
});
