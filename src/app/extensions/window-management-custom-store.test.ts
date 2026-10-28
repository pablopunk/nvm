// biome-ignore-all lint/suspicious/noExplicitAny: fake extension contexts mirror the untyped runtime bridge.
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  CUSTOM_WINDOW_COMMANDS_KEY,
  extensionWindowCustomCommandsStore,
  MAX_CUSTOM_WINDOW_COMMAND_TITLE_LENGTH,
  MAX_CUSTOM_WINDOW_COMMANDS,
  normalizeCustomWindowCommands,
} from './window-management-custom-store';
import { memoryExtensionStorage } from './window-management-test-doubles';

const OVERFLOW_COUNT = 5;

const VALID_LEFT_THIRD = {
  id: 'custom-left-third',
  title: 'Left third',
  x: { value: 0, unit: 'points' as const },
  width: { value: 33, unit: 'percent' as const },
};

test('keeps well-formed entries and trims their titles', () => {
  assert.deepEqual(
    normalizeCustomWindowCommands([
      { ...VALID_LEFT_THIRD, title: '  Left third  ' },
    ]),
    [{ ...VALID_LEFT_THIRD, title: 'Left third' }],
  );
});

test('drops entries that are not objects or lack an id or title', () => {
  const normalized = normalizeCustomWindowCommands([
    null,
    'nope',
    { ...VALID_LEFT_THIRD, id: '' },
    { ...VALID_LEFT_THIRD, title: '   ' },
    {
      ...VALID_LEFT_THIRD,
      title: 'x'.repeat(MAX_CUSTOM_WINDOW_COMMAND_TITLE_LENGTH + 1),
    },
    VALID_LEFT_THIRD,
  ]);
  assert.deepEqual(normalized, [VALID_LEFT_THIRD]);
});

test('drops entries without any dimension', () => {
  assert.deepEqual(
    normalizeCustomWindowCommands([{ id: 'empty', title: 'Empty' }]),
    [],
  );
});

test('drops a whole entry when any stored dimension is out of range', () => {
  const normalized = normalizeCustomWindowCommands([
    { id: 'bad-width', title: 'Bad', width: { value: 0, unit: 'percent' } },
    { id: 'bad-unit', title: 'Bad', width: { value: 10, unit: 'inches' } },
    { id: 'bad-offset', title: 'Bad', y: { value: 101, unit: 'percent' } },
    {
      id: 'mixed',
      title: 'Mixed',
      x: { value: 10, unit: 'points' },
      y: { value: 'ten', unit: 'points' },
    },
  ]);
  assert.deepEqual(normalized, []);
});

test('keeps the first entry when ids repeat', () => {
  const normalized = normalizeCustomWindowCommands([
    VALID_LEFT_THIRD,
    { ...VALID_LEFT_THIRD, title: 'Duplicate' },
  ]);
  assert.deepEqual(normalized, [VALID_LEFT_THIRD]);
});

test('caps the list at the maximum number of custom commands', () => {
  const raw = Array.from(
    { length: MAX_CUSTOM_WINDOW_COMMANDS + OVERFLOW_COUNT },
    (_, index) => ({
      id: `custom-${index}`,
      title: `Command ${index}`,
      width: { value: 50, unit: 'percent' },
    }),
  );
  const normalized = normalizeCustomWindowCommands(raw);
  assert.equal(normalized.length, MAX_CUSTOM_WINDOW_COMMANDS);
  assert.equal(
    normalized.at(-1)?.id,
    `custom-${MAX_CUSTOM_WINDOW_COMMANDS - 1}`,
  );
});

test('non-array storage reads as an empty list', () => {
  assert.deepEqual(normalizeCustomWindowCommands({ id: 'x' }), []);
  assert.deepEqual(normalizeCustomWindowCommands(undefined), []);
});

test('store round trip reads back what was saved under the stable key', async () => {
  const storage = memoryExtensionStorage();
  const store = extensionWindowCustomCommandsStore({ storage } as any);

  await store.save([VALID_LEFT_THIRD]);

  assert.deepEqual(storage.values.get(CUSTOM_WINDOW_COMMANDS_KEY), [
    VALID_LEFT_THIRD,
  ]);
  assert.deepEqual(await store.load(), [VALID_LEFT_THIRD]);
});

test('store save normalizes invalid entries before persisting', async () => {
  const storage = memoryExtensionStorage();
  const store = extensionWindowCustomCommandsStore({ storage } as any);

  await store.save([
    { id: 'no-dimension', title: 'Nothing' } as any,
    VALID_LEFT_THIRD,
  ]);

  assert.deepEqual(await store.load(), [VALID_LEFT_THIRD]);
});
