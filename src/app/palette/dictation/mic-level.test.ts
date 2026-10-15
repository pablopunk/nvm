import assert from 'node:assert/strict';
import test from 'node:test';
import {
  clampMicLevel,
  micLevelDisplayValue,
  smoothMicLevel,
} from './mic-level';

test('clamps levels to the 0..1 range', () => {
  assert.equal(clampMicLevel(-0.5), 0);
  assert.equal(clampMicLevel(0.4), 0.4);
  assert.equal(clampMicLevel(2), 1);
  assert.equal(clampMicLevel(Number.NaN), 0);
  assert.equal(clampMicLevel(Number.POSITIVE_INFINITY), 0);
});

test('expands quiet signals with a square-root curve', () => {
  assert.equal(micLevelDisplayValue(0), 0);
  assert.equal(micLevelDisplayValue(1), 1);
  assert.equal(micLevelDisplayValue(0.25), 0.5);
  assert.equal(micLevelDisplayValue(4), 1);
});

test('attacks fast and decays slowly', () => {
  assert.equal(smoothMicLevel(0.1, 0.8), micLevelDisplayValue(0.8));
  assert.equal(smoothMicLevel(0.8, 0), 0.8 * 0.6);
  assert.equal(smoothMicLevel(0, 0), 0);
});
