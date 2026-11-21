export type PaletteItemKind = 'command' | 'app' | 'file' | 'clipboard';

/** The tile tones of the 2.x palette (`Tones` in src/native/internal/design/tokens.go). */
export type PaletteTone = 'yellow' | 'blue' | 'purple' | 'green' | 'red' | 'orange';

export interface PaletteItem {
  id: string;
  title: string;
  subtitle: string;
  /** Lucide icon name in kebab-case, as used by the desktop app. */
  icon?: string;
  /** Color of the tile's glyph, as the 2.x row's appearance sets it. */
  tone?: PaletteTone;
  /** Title of the Enter action when it is not the kind's default. */
  primary?: string;
  aliases: string[];
  kind: PaletteItemKind;
  /** Bundled app icon path; replaces the generic Lucide glyph. */
  image?: string;
  /** Site opened in a new tab on Enter; items without one show the download notice. */
  url?: string;
  /** Extra text that makes the item findable without being shown. */
  keywords?: string[];
}
