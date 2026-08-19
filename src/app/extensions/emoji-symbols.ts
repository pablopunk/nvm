import {
  CHARACTER_RECORDS,
  characterCodePoints,
  characterId,
  characterWithSkinTone,
  type CharacterCatalogRecord,
} from '../electron/emoji-symbol-catalog';

type CharacterPickerState = {
  category: string;
};

const ALL_CATEGORY = 'all';
const MAX_VISIBLE_ITEMS = 160;
const SKIN_TONES = [
  ['Default', ''],
  ['Light', '🏻'],
  ['Medium-light', '🏼'],
  ['Medium', '🏽'],
  ['Medium-dark', '🏾'],
  ['Dark', '🏿'],
] as const;

function titleCase(value: string) {
  return value.replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function codePoints(glyph: string) {
  return characterCodePoints(glyph);
}

const CATEGORY_OPTIONS = [
  { title: 'All Characters', value: ALL_CATEGORY },
  ...Array.from(new Set(CHARACTER_RECORDS.map((record) => record.group))).map(
    (group) => ({ title: group, value: group }),
  ),
];

function normalizedPickerState(value: unknown): CharacterPickerState {
  const stored = value && typeof value === 'object' ? (value as any) : {};
  const validCategory = CATEGORY_OPTIONS.some(
    (option) => option.value === stored.category,
  );
  return {
    category: validCategory ? stored.category : ALL_CATEGORY,
  };
}

async function readPickerState(ctx: any) {
  return normalizedPickerState(await ctx.storage.get('pickerState', {}));
}

async function savePickerState(ctx: any, state: CharacterPickerState) {
  await ctx.storage.set('pickerState', state);
}

function orderedRecords(state: CharacterPickerState) {
  return state.category === ALL_CATEGORY
    ? CHARACTER_RECORDS
    : CHARACTER_RECORDS.filter((record) => record.group === state.category);
}

function applySkinTone(glyph: string, skinTone: string) {
  const skinToneIndex = SKIN_TONES.findIndex((entry) => entry[1] === skinTone);
  return characterWithSkinTone(glyph, Math.max(0, skinToneIndex));
}

function pasteAction(record: CharacterCatalogRecord, skinTone = 0) {
  return {
    type: 'insertCharacter',
    characterId: characterId(record),
    mode: 'paste',
    skinTone,
    title: `Paste ${titleCase(record.name)}`,
    dismissAfterRun: 'auto',
  };
}

function copyAction(record: CharacterCatalogRecord, skinTone = 0) {
  return {
    type: 'insertCharacter',
    characterId: characterId(record),
    mode: 'copy',
    skinTone,
    title: `Copy ${titleCase(record.name)}`,
    shortcut: 'Command+Enter',
    dismissAfterRun: 'auto',
  };
}

function skinToneAction(
  record: CharacterCatalogRecord,
  mode: 'copy' | 'paste',
) {
  return {
    type: 'submenu',
    title: `${mode === 'paste' ? 'Paste' : 'Copy'} with Skin Tone`,
    icon: 'hand',
    submenu: {
      title: 'Skin Tone',
      sections: [
        {
          actions: SKIN_TONES.map((_skinTone, index) => ({
            ...(mode === 'paste'
              ? pasteAction(record, index)
              : copyAction(record, index)),
            title: `${SKIN_TONES[index][0]} ${applySkinTone(record.glyph, SKIN_TONES[index][1])}`,
          })),
        },
      ],
    },
  };
}

function characterItem(ctx: any, record: CharacterCatalogRecord) {
  const canPaste = ctx.system.capabilities.has('frontmost-paste');
  const skinToneMode = canPaste ? 'paste' : 'copy';
  const primaryAction = canPaste ? pasteAction(record) : copyAction(record);
  const actions = [
    canPaste ? copyAction(record) : null,
    record.supportsSkinTone ? skinToneAction(record, skinToneMode) : null,
    {
      type: 'insertCharacter',
      characterId: characterId(record),
      mode: 'copyUnicode',
      title: 'Copy Unicode',
      shortcut: 'Command+Shift+Alt+C',
      dismissAfterRun: 'auto',
    },
  ].filter(Boolean);
  return ctx.ui.item({
    id: characterId(record),
    title: titleCase(record.name),
    glyph: record.glyph,
    keywords: [
      record.glyph,
      record.name,
      record.group,
      codePoints(record.glyph),
      ...record.keywords,
    ],
    primaryAction,
    actions,
    actionPanel: { title: record.name, sections: [{ actions }] },
  });
}

async function pickerView(ctx: any, providedState?: CharacterPickerState) {
  const state = providedState || (await readPickerState(ctx));
  const items = orderedRecords(state).map((record) =>
    characterItem(ctx, record),
  );
  return ctx.ui.grid({
    id: 'emoji-symbols',
    title: 'Emoji & Symbols',
    subtitle:
      'Enter to paste. Command+Enter to copy. Search to narrow the catalog.',
    searchBarPlaceholder: 'Search emoji, symbols, or Unicode values',
    layout: 'compact',
    columns: 8,
    maxVisibleItems: MAX_VISIBLE_ITEMS,
    items,
    searchAccessory: {
      id: 'category',
      tooltip: 'Character category',
      value: state.category,
      items: CATEGORY_OPTIONS,
      onChange: ctx.actions.run(
        'Change Category',
        async (innerCtx: any, action: any) => {
          const current = await readPickerState(innerCtx);
          current.category = String(action.value || ALL_CATEGORY);
          await savePickerState(innerCtx, current);
          return innerCtx.navigation.replace(
            await pickerView(innerCtx, current),
          );
        },
      ),
    },
    emptyView: {
      title: 'No matching characters',
      subtitle: 'Try a name, keyword, character, or Unicode value.',
    },
  });
}

function openPicker(ctx: any) {
  return pickerView(ctx);
}

export function createEmojiSymbolsExtension() {
  return {
    id: 'nevermind.emoji-symbols',
    title: 'Emoji & Symbols',
    commands: [
      {
        id: 'emoji-symbols',
        title: 'Emoji & Symbols',
        subtitle: 'Search and paste emoji, symbols, and Unicode characters',
        aliases: [
          'emoji picker',
          'symbol picker',
          'unicode characters',
          'special characters',
        ],
        icon: 'smile-plus',
        score: 18,
        run: openPicker,
      },
    ],
  };
}
