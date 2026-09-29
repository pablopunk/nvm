// biome-ignore-all lint/style/useNamingConvention: Electron accelerator tokens are canonical title-case external values.
import { isReservedPaletteAccelerator } from './os';
import { formatShortcut, normalizeAccelerator } from '../shared/shortcut-utils';

export { formatShortcut, normalizeAccelerator };

export function isSpotlightAccelerator(accelerator: unknown) {
  return isReservedPaletteAccelerator(normalizeAccelerator(accelerator));
}
