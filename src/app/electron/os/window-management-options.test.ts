import assert from 'node:assert/strict';
import test from 'node:test';
import {
  resolveWindowManagementOptions,
  windowGapsFrom,
} from './window-management-options';
import {
  DEFAULT_WINDOW_MANAGEMENT_OPTIONS,
  type WindowManagementOptions,
} from './window-management-types';

const DEFAULTS = DEFAULT_WINDOW_MANAGEMENT_OPTIONS;

test('no partial options resolves to the defaults', () => {
  assert.deepEqual(resolveWindowManagementOptions(), DEFAULTS);
  assert.deepEqual(resolveWindowManagementOptions({}), DEFAULTS);
});

test('a partial outer gap overrides only the edges it names', () => {
  const options = resolveWindowManagementOptions({
    outerGap: { right: 20, bottom: 0 },
  });
  assert.deepEqual(options.outerGap, { top: 8, right: 20, bottom: 0, left: 8 });
  assert.equal(options.innerGap, DEFAULTS.innerGap);
});

test('scalar options override their defaults', () => {
  const options = resolveWindowManagementOptions({
    innerGap: 12,
    applyGapToMaximize: false,
    cycle: false,
    cycleOrder: 'half-third-two-thirds',
    displayOrder: 'top-to-bottom',
    displayMoveSizing: 'keep-size',
  });
  assert.deepEqual(options, {
    outerGap: DEFAULTS.outerGap,
    innerGap: 12,
    applyGapToMaximize: false,
    cycle: false,
    cycleOrder: 'half-third-two-thirds',
    displayOrder: 'top-to-bottom',
    displayMoveSizing: 'keep-size',
  });
});

test('explicit undefined fields fall back to the defaults', () => {
  const options = resolveWindowManagementOptions({
    innerGap: undefined,
    outerGap: { top: undefined },
    cycle: undefined,
  });
  assert.deepEqual(options, DEFAULTS);
});

test('resolving never mutates the shared defaults or the partial input', () => {
  const partial = { outerGap: { left: 30 } };
  const options = resolveWindowManagementOptions(partial);
  options.outerGap.top = 99;
  assert.equal(DEFAULTS.outerGap.top, 8);
  assert.deepEqual(partial, { outerGap: { left: 30 } });
});

test('windowGapsFrom maps the options onto the engine gaps', () => {
  const options: WindowManagementOptions = resolveWindowManagementOptions({
    outerGap: { top: 1, right: 2, bottom: 3, left: 4 },
    innerGap: 5,
    applyGapToMaximize: false,
  });
  assert.deepEqual(windowGapsFrom(options), {
    outer: { top: 1, right: 2, bottom: 3, left: 4 },
    inner: 5,
    applyToMaximize: false,
  });
});
