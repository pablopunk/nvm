import assert from 'node:assert/strict';
import test from 'node:test';
import { acceleratorFromKeyboardEvent } from './shortcuts';

function keyboardEvent(
  init: Partial<{
    key: string;
    code: string;
    metaKey: boolean;
    ctrlKey: boolean;
    altKey: boolean;
    shiftKey: boolean;
  }> & { modifierState?: Record<string, boolean> } = {},
) {
  const { modifierState = {}, ...rest } = init;
  return {
    key: '',
    code: '',
    metaKey: false,
    ctrlKey: false,
    altKey: false,
    shiftKey: false,
    ...rest,
    getModifierState: (name: string) => Boolean(modifierState[name]),
  };
}

test('acceleratorFromKeyboardEvent builds accelerators from modifier flags', () => {
  assert.equal(
    acceleratorFromKeyboardEvent(
      keyboardEvent({ key: ' ', code: 'Space', ctrlKey: true }),
    ),
    'Control+Space',
  );
  assert.equal(
    acceleratorFromKeyboardEvent(
      keyboardEvent({ key: ' ', code: 'Space', shiftKey: true }),
    ),
    'Shift+Space',
  );
  assert.equal(
    acceleratorFromKeyboardEvent(
      keyboardEvent({
        key: 'k',
        code: 'KeyK',
        metaKey: true,
        altKey: true,
      }),
    ),
    'Command+Alt+K',
  );
});

test('acceleratorFromKeyboardEvent recovers AltGraph modifiers cleared by Chromium on Windows', () => {
  assert.equal(
    acceleratorFromKeyboardEvent(
      keyboardEvent({
        key: ' ',
        code: 'Space',
        modifierState: { AltGraph: true },
      }),
      true,
    ),
    'Control+Alt+Space',
  );
});

test('acceleratorFromKeyboardEvent leaves AltGraph alone off Windows', () => {
  assert.equal(
    acceleratorFromKeyboardEvent(
      keyboardEvent({
        key: ' ',
        code: 'Space',
        modifierState: { AltGraph: true },
      }),
      false,
    ),
    '',
  );
});

test('acceleratorFromKeyboardEvent ignores bare keys and lone modifiers', () => {
  assert.equal(
    acceleratorFromKeyboardEvent(keyboardEvent({ key: 'g', code: 'KeyG' })),
    '',
  );
  assert.equal(
    acceleratorFromKeyboardEvent(
      keyboardEvent({ key: 'Control', code: 'ControlLeft', ctrlKey: true }),
    ),
    '',
  );
});
