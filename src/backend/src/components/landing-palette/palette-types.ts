export type PaletteItemKind = 'command' | 'app' | 'file' | 'clipboard';

export interface PaletteItem {
  id: string;
  title: string;
  subtitle: string;
  /** Lucide icon name in kebab-case, as used by the desktop app. */
  icon?: string;
  aliases: string[];
  kind: PaletteItemKind;
  /** Bundled app icon path; replaces the generic Lucide glyph. */
  image?: string;
  /** Site opened in a new tab on Enter; items without one show the download notice. */
  url?: string;
  /** Extra text that makes the item findable without being shown. */
  keywords?: string[];
}
