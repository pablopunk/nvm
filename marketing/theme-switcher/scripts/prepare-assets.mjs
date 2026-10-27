import { execFileSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { copyFile, mkdir, readFile, writeFile } from 'node:fs/promises';
import { homedir } from 'node:os';
import { resolve } from 'node:path';

const require = createRequire(import.meta.url);
const sharp = require('sharp');
const output = new URL('../public/', import.meta.url);
const themes = JSON.parse(
  await readFile(new URL('../src/themes.json', import.meta.url), 'utf8'),
);
await mkdir(output, { recursive: true });
await copyFile(
  new URL('../../../src/backend/public/wordmark.png', import.meta.url),
  new URL('wordmark.png', output),
);

const sources = [
  ['meadow', 'online/wallhaven-213edy.png', 'Blue Meadow'],
  ['pink', 'pexels-marina-m-8357330.jpg', 'Neon Interior'],
  ['forest', 'priscilla-du-preez-B9LlLJYmw74-unsplash.jpg', 'Misty Forest'],
  ['mountain', 'pexels-eberhardgross-1428277.jpg', 'Moonlit Mountains'],
  ['range-dark', 'range-dark.jpg', 'Midnight Ranges'],
  ['range-light', 'range-light.jpg', 'Morning Ranges'],
  ['fuji', 'fuji.jpg', 'Mount Fuji'],
  ['kanagawa', 'kanagawa.jpg', 'The Great Wave'],
  ['audi', 'audi.jpg', 'Desert Drive'],
];
const manifest = [];
for (const [id, filename, title] of sources) {
  const source = resolve(homedir(), '.config/wallpapers', filename);
  await sharp(source)
    .resize(2560, 1440, { fit: 'cover' })
    .jpeg({ quality: 90 })
    .toFile(new URL(`${id}.jpg`, output).pathname);
  manifest.push({ id, title, source, asset: `${id}.jpg` });
}
const dockIcons = [
  [
    'finder',
    '/System/Library/CoreServices/Finder.app/Contents/Resources/Finder.icns',
  ],
  [
    'calendar',
    '/System/Applications/Calendar.app/Contents/Resources/AppIcon.icns',
  ],
  ['notes', '/System/Applications/Notes.app/Contents/Resources/AppIcon.icns'],
  ['photos', '/System/Applications/Photos.app/Contents/Resources/AppIcon.icns'],
  [
    'calculator',
    '/System/Applications/Calculator.app/Contents/Resources/AppIcon.icns',
  ],
  [
    'settings',
    '/System/Applications/System Settings.app/Contents/Resources/Settings.icns',
  ],
  [
    'nevermind',
    new URL('../../../src/backend/public/icon.png', import.meta.url).pathname,
  ],
];
for (const [id, source] of dockIcons) {
  const prepared = new URL(`app-${id}.png`, output).pathname;
  if (source.endsWith('.icns')) {
    execFileSync('sips', ['-s', 'format', 'png', source, '--out', prepared], {
      stdio: 'ignore',
    });
  } else {
    await copyFile(source, prepared);
  }
  const iconSize = id === 'nevermind' ? 62 : 76;
  const padding = (76 - iconSize) / 2;
  const roundedIcon = await sharp(prepared)
    .resize(iconSize, iconSize, {
      fit: 'contain',
      background: { r: 0, g: 0, b: 0, alpha: 0 },
    })
    .ensureAlpha()
    .composite([
      {
        input: Buffer.from(
          `<svg width="${iconSize}" height="${iconSize}"><rect width="${iconSize}" height="${iconSize}" rx="${(18 * iconSize) / 76}" fill="white"/></svg>`,
        ),
        blend: 'dest-in',
      },
    ])
    .png()
    .toBuffer();
  const sourceIcon = await sharp(roundedIcon)
    .extend({
      top: padding,
      bottom: padding,
      left: padding,
      right: padding,
      background: { r: 0, g: 0, b: 0, alpha: 0 },
    })
    .png()
    .toBuffer();
  for (const theme of themes) {
    await sharp(sourceIcon)
      .tint(theme.tint)
      .png()
      .toFile(new URL(`dock-${id}-${theme.id}.png`, output).pathname);
  }
}
await writeFile(
  new URL('assets.json', output),
  JSON.stringify(
    {
      wallpapers: manifest,
      dockIcons: dockIcons.map(([id, source]) => ({ id, source })),
    },
    null,
    2,
  ),
);
console.log(
  `Prepared ${sources.length} wallpapers and ${dockIcons.length} default Dock icons.`,
);
