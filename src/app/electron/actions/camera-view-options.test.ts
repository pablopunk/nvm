import assert from 'node:assert/strict';
import { test } from 'node:test';
import { normalizeCameraViewOptions } from './camera-view-options';

test('camera view options default to an unmirrored cover frame', () => {
  assert.deepEqual(normalizeCameraViewOptions({}), {
    mirrored: false,
    framing: 'cover',
  });
});

test('camera view options accept only the documented values', () => {
  assert.deepEqual(
    normalizeCameraViewOptions({ mirrored: true, framing: 'contain' }),
    { mirrored: true, framing: 'contain' },
  );
  assert.deepEqual(
    normalizeCameraViewOptions({ mirrored: 'yes', framing: 'stretch' }),
    { mirrored: false, framing: 'cover' },
  );
});

test('normalized camera options stay clone-safe', () => {
  const options = normalizeCameraViewOptions({
    mirrored: true,
    framing: 'contain',
    run: () => undefined,
  });
  assert.doesNotThrow(() => structuredClone(options));
  assert.equal('run' in options, false);
});
