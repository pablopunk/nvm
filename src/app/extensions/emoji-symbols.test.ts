import assert from 'node:assert/strict';
import test from 'node:test';
import { createEmojiSymbolsExtension } from './emoji-symbols';

function actionFactory(title: string, handler: unknown, options = {}) {
  return { ...options, type: 'runExtensionAction', title, __handler: handler };
}

function pickerContext(canPaste = true) {
  let pickerState: unknown = {};
  return {
    actions: {
      run: actionFactory,
    },
    navigation: {
      run: (action: unknown) => ({ action }),
      replace: (view: unknown) => ({ view, navigation: 'replace' }),
    },
    storage: {
      get: async () => pickerState,
      set: async (_key: string, value: unknown) => {
        pickerState = value;
      },
    },
    system: { capabilities: { has: () => canPaste } },
    ui: {
      grid: (view: unknown) => ({ ...(view as object), type: 'grid' }),
      item: (item: unknown) => item,
    },
  };
}

async function pickerView(context: any) {
  const command = createEmojiSymbolsExtension().commands[0];
  return (await command.run(context)) as any;
}

test('exposes a searchable grid with emoji and Unicode symbols', async () => {
  const view = await pickerView(pickerContext());
  const grinning = view.items.find((item: any) => item.glyph === '😀');
  const infinity = view.items.find((item: any) => item.glyph === '∞');
  const checkMark = view.items.find((item: any) => item.glyph === '✓');

  assert.equal(view.type, 'grid');
  assert.equal(view.columns, 8);
  assert.equal(view.maxVisibleItems, 160);
  assert.equal(view.searchAccessory.value, 'all');
  assert.ok(view.items.length > 2_000);
  assert.equal(grinning.title, 'Grinning Face');
  assert.equal(grinning.subtitle, undefined);
  assert.ok(grinning.keywords.includes('smile'));
  assert.equal(infinity.title, 'Infinity');
  assert.ok(infinity.keywords.includes('U+221E'));
  assert.equal(checkMark.title, 'Check Mark');
});

test('pastes by default and exposes copy, skin tone, and Unicode actions', async () => {
  const view = await pickerView(pickerContext());
  const waving = view.items.find((item: any) => item.glyph === '👋');
  const actionTitles = waving.actions.map((action: any) => action.title);
  const skinTone = waving.actions.find(
    (action: any) => action.title === 'Paste with Skin Tone',
  );

  assert.equal(waving.primaryAction.title, 'Paste Waving Hand');
  assert.equal(waving.primaryAction.type, 'insertCharacter');
  assert.equal(waving.primaryAction.mode, 'paste');
  assert.deepEqual(actionTitles, [
    'Copy Waving Hand',
    'Paste with Skin Tone',
    'Copy Unicode',
  ]);
  assert.deepEqual(
    skinTone.submenu.sections[0].actions.map((action: any) => action.title),
    [
      'Default 👋',
      'Light 👋🏻',
      'Medium-light 👋🏼',
      'Medium 👋🏽',
      'Medium-dark 👋🏾',
      'Dark 👋🏿',
    ],
  );
  assert.equal(waving.actions.at(-1).mode, 'copyUnicode');
});

test('falls back to copy when frontmost paste is unavailable', async () => {
  const view = await pickerView(pickerContext(false));
  const waving = view.items.find((item: any) => item.glyph === '👋');

  assert.equal(waving.primaryAction.title, 'Copy Waving Hand');
  assert.equal(waving.primaryAction.type, 'insertCharacter');
  assert.equal(waving.primaryAction.mode, 'copy');
  assert.equal(
    waving.actions.some(
      (action: any) => action.title === 'Copy with Skin Tone',
    ),
    true,
  );
});

test('category changes persist and replace the current picker view', async () => {
  const context = pickerContext();
  const view = await pickerView(context);
  const changeCategory = view.searchAccessory.onChange.__handler;
  const result = (await changeCategory(context, { value: 'Currency' })) as any;

  assert.equal(result.navigation, 'replace');
  assert.equal(result.view.searchAccessory.value, 'Currency');
  assert.ok(result.view.items.length > 10);
  assert.equal(
    result.view.items.every((item: any) => item.keywords.includes('Currency')),
    true,
  );
});

test('skin tones precede variation selectors in canonical emoji sequences', async () => {
  const view = await pickerView(pickerContext());
  const pointing = view.items.find((item: any) => item.glyph === '☝️');
  const skinTone = pointing.actions.find(
    (action: any) => action.title === 'Paste with Skin Tone',
  );

  assert.equal(skinTone.submenu.sections[0].actions[1].title, 'Light ☝🏻');
  assert.equal(skinTone.submenu.sections[0].actions[1].skinTone, 1);
});
