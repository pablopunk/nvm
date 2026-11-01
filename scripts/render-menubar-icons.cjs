#!/usr/bin/env node
// Rasterizes menu bar / tray glyphs into src/app/resources/menubar-icons.
// Native status areas cannot load SVG, so the PNGs are committed.
// Needs `rsvg-convert` (librsvg) and `magick` (ImageMagick) on PATH:
//   node scripts/render-menubar-icons.cjs
const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

const ROOT = path.resolve(__dirname, '..');
const ICON_LIST = path.join(ROOT, 'src/app/shared/menubar-icons.ts');
const LUCIDE_DIR = path.join(ROOT, 'node_modules/lucide-react/dist/esm/icons');
const SOURCES_DIR = path.join(__dirname, 'menubar-icon-sources');
const APP_ICON = path.join(ROOT, 'build/Icon.icon/Assets/icon.png');
const OUT_DIR = path.join(ROOT, 'src/app/resources/menubar-icons');
const POINT_SIZE = 16;
const SCALES = [1, 2];
const BLACK = '#000000';
const WHITE = '#ffffff';
const BRAND_GOLD = '#F5B400';
const TMP_DIR = fs.mkdtempSync(path.join(os.tmpdir(), 'menubar-icons-'));

function curatedIconNames() {
  const source = fs.readFileSync(ICON_LIST, 'utf8');
  const list = source.match(/MENUBAR_ICON_NAMES = \[([\s\S]*?)\] as const/);
  if (!list) throw new Error('MENUBAR_ICON_NAMES not found');
  return [...list[1].matchAll(/'([a-z0-9-]+)'/g)].map((match) => match[1]);
}

async function lucideIconNode(name) {
  const file = path.join(LUCIDE_DIR, `${name}.mjs`);
  const module = await import(pathToFileURL(file).href);
  if (module.__iconData?.node) return module.__iconData.node;
  const alias = fs
    .readFileSync(file, 'utf8')
    .match(/export \{ default \} from '\.\/([a-z0-9-]+)\.mjs'/);
  if (alias) return lucideIconNode(alias[1]);
  throw new Error(`Lucide icon data not found for ${name}`);
}

function lucideSvg(iconNode, color, filled = false) {
  const children = iconNode
    .map(([tag, attributes]) => {
      const attrs = Object.entries(attributes)
        .filter(([key]) => key !== 'key')
        .map(([key, value]) => `${key}="${value}"`)
        .join(' ');
      return `<${tag} ${attrs}/>`;
    })
    .join('');
  return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="${filled ? color : 'none'}" stroke="${color}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${children}</svg>`;
}

function outputFile(name, scale) {
  return path.join(OUT_DIR, `${name}${scale === 1 ? '' : `@${scale}x`}.png`);
}

function rasterizeSvgFile(svgFile, name) {
  for (const scale of SCALES) {
    const size = String(POINT_SIZE * scale);
    execFileSync('rsvg-convert', [
      '-w',
      size,
      '-h',
      size,
      '-o',
      outputFile(name, scale),
      svgFile,
    ]);
  }
}

function rasterizeSvg(svg, name) {
  const svgFile = path.join(TMP_DIR, `${name}.svg`);
  fs.writeFileSync(svgFile, svg);
  rasterizeSvgFile(svgFile, name);
}

function magickForEachScale(name, argsForSize) {
  for (const scale of SCALES)
    execFileSync('magick', [
      ...argsForSize(POINT_SIZE * scale),
      outputFile(name, scale),
    ]);
}

/** Downscaled app icon with macOS-like rounded corners. */
function renderAppIconTile(name) {
  magickForEachScale(name, (size) => [
    APP_ICON,
    '-resize',
    `${size}x${size}`,
    '(',
    '-size',
    `${size}x${size}`,
    'xc:none',
    '-fill',
    'white',
    '-draw',
    `roundrectangle 0,0 ${size - 1},${size - 1} ${size * 0.22},${size * 0.22}`,
    ')',
    '-compose',
    'DstIn',
    '-composite',
  ]);
}

/** The app icon's glossy bolt with its black backdrop turned transparent. */
function renderGlossyBolt(name) {
  magickForEachScale(name, (size) => [
    APP_ICON,
    '-gravity',
    'center',
    '-crop',
    '86%x86%+0+0',
    '+repage',
    '(',
    '+clone',
    '-colorspace',
    'gray',
    '-level',
    '27%,60%',
    ')',
    '-alpha',
    'off',
    '-compose',
    'CopyOpacity',
    '-composite',
    '-resize',
    `${size}x${size}`,
  ]);
}

async function renderCuratedIcons() {
  const names = curatedIconNames();
  for (const name of names) {
    const iconNode = await lucideIconNode(name);
    rasterizeSvg(lucideSvg(iconNode, BLACK), name);
    rasterizeSvg(lucideSvg(iconNode, WHITE), `${name}-white`);
  }
  return names.length;
}

async function renderAppStyles() {
  const bolt = await lucideIconNode('zap');
  rasterizeSvg(lucideSvg(bolt, BLACK, true), 'nevermind-bolt-filled');
  rasterizeSvg(lucideSvg(bolt, WHITE, true), 'nevermind-bolt-filled-white');
  rasterizeSvg(lucideSvg(bolt, BLACK), 'nevermind-bolt-outline');
  rasterizeSvg(lucideSvg(bolt, WHITE), 'nevermind-bolt-outline-white');
  rasterizeSvg(lucideSvg(bolt, BRAND_GOLD), 'nevermind-gold-outline');
  rasterizeSvg(lucideSvg(bolt, BRAND_GOLD, true), 'nevermind-gold-filled');
  rasterizeSvgFile(
    path.join(SOURCES_DIR, 'noto-emoji-u26a1.svg'),
    'nevermind-emoji',
  );
  renderAppIconTile('nevermind-app-icon');
  renderGlossyBolt('nevermind-gold-glossy');
}

async function renderAll() {
  fs.rmSync(OUT_DIR, { recursive: true, force: true });
  fs.mkdirSync(OUT_DIR, { recursive: true });
  const curated = await renderCuratedIcons();
  await renderAppStyles();
  console.log(
    `Rendered ${curated} curated icons and 7 app styles into ${OUT_DIR}`,
  );
}

renderAll()
  .catch((error) => {
    console.error(error);
    process.exitCode = 1;
  })
  .finally(() => fs.rmSync(TMP_DIR, { recursive: true, force: true }));
