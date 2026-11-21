import { Command } from 'cmdk';
import { CornerDownLeft, Zap } from 'lucide-react';
import { isAutomate } from './palette-actions';
import { RELEASES_URL } from './palette-data';
import { iconNamed } from './palette-icons';
import type { PaletteItem } from './palette-types';

export const DOWNLOAD_NOTICE_TEXT = 'Download the app to actually do that!';

/** The 2.x row tile: the app's own icon, the gold Automate tile, or a glyph on a small keyed tile. */
function Tile({ item }: { item: PaletteItem }) {
  if (item.image) {
    return (
      <span className="tile app">
        <img src={item.image} alt="" width={40} height={40} decoding="async" />
      </span>
    );
  }
  if (isAutomate(item)) {
    return (
      <span className="tile key prom">
        <Zap className="i" aria-hidden="true" />
      </span>
    );
  }
  const Icon = iconNamed(item.icon);
  return (
    <span className={item.tone ? `tile key tone-${item.tone}` : 'tile key'}>
      <Icon className="i" aria-hidden="true" />
    </span>
  );
}

function rowClass(item: PaletteItem, selected: boolean) {
  const classes = ['row'];
  if (isAutomate(item)) classes.push('prom', 'tint-yellow');
  if (selected) classes.push('sel');
  return classes.join(' ');
}

/** One result: Row in src/native/internal/design/row.go, with ↵ on the selected plate. */
export function PaletteRow({
  item,
  selected,
  showNotice,
  onSelect,
}: {
  item: PaletteItem;
  selected: boolean;
  showNotice: boolean;
  onSelect: () => void;
}) {
  return (
    <Command.Item value={item.id} className={rowClass(item, selected)} onSelect={onSelect}>
      <Tile item={item} />
      <span className="tx">
        <span className="t">{item.title}</span>
        {showNotice ? (
          <span className="s notice" role="status">
            <a href={RELEASES_URL}>{DOWNLOAD_NOTICE_TEXT}</a>
          </span>
        ) : item.subtitle ? (
          <span className="s">{item.subtitle}</span>
        ) : null}
      </span>
      <CornerDownLeft className="i enter" aria-label="Enter" />
    </Command.Item>
  );
}
