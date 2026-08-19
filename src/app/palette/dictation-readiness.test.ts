import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createMicrophoneReadinessTracker,
  needsBluetoothMicrophoneReadiness,
} from './dictation-readiness';

test('identifies Bluetooth microphones by low capture rate or device name', () => {
  assert.equal(needsBluetoothMicrophoneReadiness('AirPols', 24_000), true);
  assert.equal(needsBluetoothMicrophoneReadiness('Jabra Speak', 48_000), true);
  assert.equal(
    needsBluetoothMicrophoneReadiness('MacBook Pro Microphone', 48_000),
    false,
  );
});

test('accepts the first frame from a regular microphone', () => {
  const readiness = createMicrophoneReadinessTracker(false);
  assert.equal(readiness.consume(0, false), true);
});

test('waits for stable Bluetooth signal across three current frames', () => {
  const readiness = createMicrophoneReadinessTracker(true);
  assert.equal(readiness.consume(0, false), false);
  assert.equal(readiness.consume(10, true), false);
  assert.equal(readiness.consume(20, false), false);
  assert.equal(readiness.consume(30, false), true);
});

test('resets Bluetooth signal readiness after a frame gap', () => {
  const readiness = createMicrophoneReadinessTracker(true);
  assert.equal(readiness.consume(0, true), false);
  assert.equal(readiness.consume(10, false), false);
  assert.equal(readiness.consume(300, false), false);
  assert.equal(readiness.consume(310, false), false);
});

test('accepts a continuously silent Bluetooth stream after five seconds', () => {
  const readiness = createMicrophoneReadinessTracker(true);
  assert.equal(readiness.consume(0, false), false);
  for (let now = 100; now < 5_000; now += 100)
    assert.equal(readiness.consume(now, false), false);
  assert.equal(readiness.consume(5_000, false), true);
});
