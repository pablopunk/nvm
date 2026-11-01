import assert from 'node:assert/strict';
import test from 'node:test';
import { createNativeDialogHold } from './palette-native-dialog-hold';

test('the palette stays open while its native dialog is up and regains focus after it closes', async () => {
  let refocused = 0;
  const hold = createNativeDialogHold(() => {
    refocused += 1;
  });
  let closeDialog: (paths: string[]) => void = () => {};
  const picked = hold.during(
    () =>
      new Promise<string[]>((resolve) => {
        closeDialog = resolve;
      }),
  );

  assert.equal(hold.active, true);
  assert.equal(refocused, 0);
  closeDialog(['/home/user/Trips']);

  assert.deepEqual(await picked, ['/home/user/Trips']);
  assert.equal(hold.active, false);
  assert.equal(refocused, 1);
});

test('a failed dialog still releases the palette', async () => {
  const hold = createNativeDialogHold(() => {});
  await assert.rejects(
    hold.during(() => Promise.reject(new Error('portal unavailable'))),
  );
  assert.equal(hold.active, false);
});
