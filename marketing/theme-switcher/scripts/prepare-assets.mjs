import { createRequire } from 'node:module';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { homedir } from 'node:os';
import { resolve } from 'node:path';

const require = createRequire(import.meta.url);
const sharp = require('sharp');
const output = new URL('../public/', import.meta.url);
const themes = JSON.parse(
  await readFile(new URL('../src/themes.json', import.meta.url), 'utf8'),
);
await mkdir(output, { recursive: true });

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
const screenshot = process.argv[2];
if (!screenshot)
  throw new Error(
    'Pass the supplied desktop screenshot path as the first argument.',
  );
const info = await sharp(screenshot).metadata();
if (info.width !== 2000 || info.height !== 1125)
  throw new Error('Dock crop expects the supplied 2000×1125 screenshot.');
await sharp(screenshot)
  .extract({ left: 600, top: 1057, width: 785, height: 68 })
  .png()
  .toFile(new URL('dock.png', output).pathname);
const dockIcons = [
  ['finder', 15],
  ['calculator', 61],
  ['calendar', 108],
  ['ardia', 154],
  ['photos', 200],
  ['slack', 246],
  ['t3', 292],
  ['twitter', 338],
  ['network', 384],
  ['settings', 430],
  ['zoom', 498],
  ['whatsapp', 544],
  ['chrome', 590],
];
for (const [id, left] of dockIcons) {
  const sourceIcon = await sharp(screenshot)
    .extract({ left: 601 + left, top: 1074, width: 36, height: 36 })
    .resize(76, 76)
    .ensureAlpha()
    .composite([
      {
        input: Buffer.from(
          '<svg width="76" height="76"><rect width="76" height="76" rx="18" fill="white"/></svg>',
        ),
        blend: 'dest-in',
      },
    ])
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
  JSON.stringify({ screenshot, wallpapers: manifest }, null, 2),
);
console.log(
  `Prepared ${sources.length} wallpapers, the Dock crop, and ${dockIcons.length} source icon crops.`,
);
