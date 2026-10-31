import assert from 'node:assert/strict';
import test from 'node:test';
import { mouseDownLandsOnEmptyWindowSpace } from './empty-window-space';

function elementInside(...ancestorClasses: string[]) {
  return {
    closest: (selector: string) =>
      selector
        .split(',')
        .map((part) => part.trim())
        .find((part) => ancestorClasses.includes(part.replace(/^\./, ''))),
  };
}

test('clicking the shortcut notice button does not dismiss the palette', () => {
  assert.equal(
    mouseDownLandsOnEmptyWindowSpace(
      elementInside('shortcutRegistrationNotice'),
    ),
    false,
  );
});

test('clicking inside a card or compact panel does not dismiss the palette', () => {
  assert.equal(mouseDownLandsOnEmptyWindowSpace(elementInside('card')), false);
  assert.equal(
    mouseDownLandsOnEmptyWindowSpace(
      elementInside('extensionWindowCompactPanel'),
    ),
    false,
  );
});

test('clicking outside every palette surface dismisses the palette', () => {
  assert.equal(mouseDownLandsOnEmptyWindowSpace(elementInside()), true);
  assert.equal(mouseDownLandsOnEmptyWindowSpace(null), true);
});
