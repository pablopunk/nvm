import assert from 'node:assert/strict';
import test from 'node:test';
import { createSystemThemeStore, themedImageSource } from './system-theme';

test('themed images prefer the active theme and use stable fallbacks', () => {
  const image = {
    light: 'light.png',
    dark: 'dark.png',
    src: 'default.png',
    fallback: 'fallback.png',
  };

  assert.equal(themedImageSource(image, 'light'), 'light.png');
  assert.equal(themedImageSource(image, 'dark'), 'dark.png');
  assert.equal(
    themedImageSource({ src: 'default.png' }, 'dark'),
    'default.png',
  );
  assert.equal(themedImageSource({ light: 'light.png' }, 'dark'), 'light.png');
  assert.equal(
    themedImageSource({ fallback: 'fallback.png' }, 'light'),
    'fallback.png',
  );
  assert.equal(themedImageSource('plain.png', 'dark'), 'plain.png');
  assert.equal(themedImageSource(undefined, 'light'), '');
});

test('system theme store reports changes and removes its listener', () => {
  let listener: (() => void) | undefined;
  let removedListener: (() => void) | undefined;
  const query = {
    matches: false,
    addEventListener: (_type: 'change', nextListener: () => void) => {
      listener = nextListener;
    },
    removeEventListener: (_type: 'change', nextListener: () => void) => {
      removedListener = nextListener;
    },
  };
  const store = createSystemThemeStore(() => query);
  let updates = 0;
  const unsubscribe = store.subscribe(() => {
    updates += 1;
  });

  assert.equal(store.getSnapshot(), 'light');
  query.matches = true;
  listener?.();
  assert.equal(store.getSnapshot(), 'dark');
  assert.equal(updates, 1);

  unsubscribe();
  assert.equal(removedListener, listener);
});
