import assert from 'node:assert/strict';
import test from 'node:test';
import { createWindowManagementService } from './window-management';
import type { WindowLayoutSpec } from './window-management-types';

const SELF_PID = 4242;
const LEFT_HALF: WindowLayoutSpec = {
  kind: 'fraction',
  x: 0,
  y: 0,
  width: 0.5,
  height: 1,
};

test('refuses to move Nevermind itself and never touches the helper', async () => {
  let helperCalls = 0;
  const countHelperCall = () => {
    helperCalls += 1;
    return Promise.reject(new Error('helper must not run'));
  };
  const service = createWindowManagementService({
    manager: {
      permissionState: countHelperCall,
      displays: countHelperCall,
      focusedWindow: countHelperCall,
      setFrame: countHelperCall,
      setFullScreen: countHelperCall,
    },
    ports: {
      paletteFocusTarget: () => null,
      hidePaletteAndRestoreFocus: () => Promise.resolve(true),
      frontmostTarget: () => Promise.resolve(null),
    },
    engine: {
      computeTargetFrame: () => ({ x: 0, y: 0, width: 1, height: 1 }),
      pickDisplayForFrame: () => {
        throw new Error('engine must not run');
      },
      moveFrameToDisplay: () => {
        throw new Error('engine must not run');
      },
      adjacentDisplay: () => {
        throw new Error('engine must not run');
      },
      cycleSequence: () => {
        throw new Error('engine must not run');
      },
    },
    supported: () => true,
    selfPid: SELF_PID,
  });
  const result = await service.apply(
    LEFT_HALF,
    { gap: 0, cycle: true },
    {
      source: 'shortcut',
      target: { bundleId: 'com.nevermind.app', pid: SELF_PID },
    },
  );
  assert.equal(result.ok, false);
  assert.equal(result.ok === false && result.reason, 'no-target');
  assert.equal(helperCalls, 0);
});
