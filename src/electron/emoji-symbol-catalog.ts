import {
  EMOJI_RECORDS,
  SKIN_TONE_VARIANTS,
  SYMBOL_RECORDS,
} from './extensions/emoji-symbol-data';

export type CharacterCatalogRecord = {
  glyph: string;
  name: string;
  group: string;
  keywords: readonly string[];
  supportsSkinTone: boolean;
  type: 'emoji' | 'symbol';
};

export function characterCodePoints(glyph: string) {
  return Array.from(glyph)
    .map(
      (character) =>
        `U+${character.codePointAt(0)!.toString(16).toUpperCase().padStart(4, '0')}`,
    )
    .join(' ');
}

export function characterId(record: CharacterCatalogRecord) {
  return `${record.type}:${characterCodePoints(record.glyph)}`;
}

export function characterWithSkinTone(glyph: string, skinTone: number) {
  if (!skinTone) return glyph;
  return (
    SKIN_TONE_VARIANTS[glyph]?.[skinTone - 1] ||
    SKIN_TONE_VARIANTS[glyph.replaceAll('\ufe0f', '')]?.[skinTone - 1] ||
    glyph
  );
}

export const CHARACTER_RECORDS: readonly CharacterCatalogRecord[] = [
  ...EMOJI_RECORDS.map((record) => ({
    glyph: record[0],
    name: record[1],
    group: record[2],
    keywords: record[3],
    supportsSkinTone: record[4] === 1,
    type: 'emoji' as const,
  })),
  ...SYMBOL_RECORDS.map((record) => ({
    glyph: record[0],
    name: record[1],
    group: record[2],
    keywords: [],
    supportsSkinTone: false,
    type: 'symbol' as const,
  })),
];

export const CHARACTER_RECORDS_BY_ID = new Map(
  CHARACTER_RECORDS.map((record) => [characterId(record), record]),
);
