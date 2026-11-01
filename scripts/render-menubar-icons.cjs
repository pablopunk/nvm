#!/usr/bin/env node
// Rasterizes the curated Lucide set into tray/menu bar PNGs. Native status
// areas cannot load SVG, so the PNGs are committed under src/app/resources.
// Run with Electron (it provides the canvas): `electron scripts/render-menubar-icons.cjs`.
const fs = require('node:fs');
const path = require('node:path');
const { app, BrowserWindow } = require('electron');

const ROOT = path.resolve(__dirname, '..');
const ICON_LIST = path.join(ROOT, 'src/app/shared/menubar-icons.ts');
const LUCIDE_DIR = path.join(ROOT, 'node_modules/lucide-react/dist/esm/icons');
const OUT_DIR = path.join(ROOT, 'src/app/resources/menubar-icons');
const POINT_SIZE = 16;
const VARIANTS = [
  { suffix: '', color: '#000000' },
  { suffix: '-white', color: '#ffffff' },
];

function curatedIconNames() {
  const source = fs.readFileSync(ICON_LIST, 'utf8');
  const list = source.match(/MENUBAR_ICON_NAMES = \[([\s\S]*?)\] as const/);
  if (!list) throw new Error('MENUBAR_ICON_NAMES not found');
  return [...list[1].matchAll(/'([a-z0-9-]+)'/g)].map((match) => match[1]);
}

function lucideIconNode(name) {
  const source = fs.readFileSync(path.join(LUCIDE_DIR, `${name}.mjs`), 'utf8');
  const node = source.match(/node: (\[[\s\S]*?\])\n\};/);
  if (!node) throw new Error(`Lucide icon data not found for ${name}`);
  return Function(`"use strict"; return (${node[1]});`)();
}

function svgFor(iconNode, color) {
  const children = iconNode
    .map(([tag, attributes]) => {
      const attrs = Object.entries(attributes)
        .filter(([key]) => key !== 'key')
        .map(([key, value]) => `${key}="${value}"`)
        .join(' ');
      return `<${tag} ${attrs}/>`;
    })
    .join('');
  return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="${color}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${children}</svg>`;
}

async function rasterize(win, svg, size) {
  const dataUrl = await win.webContents.executeJavaScript(`
    new Promise((resolve, reject) => {
      const image = new Image();
      image.onload = () => {
        const canvas = document.createElement('canvas');
        canvas.width = ${size};
        canvas.height = ${size};
        canvas.getContext('2d').drawImage(image, 0, 0, ${size}, ${size});
        resolve(canvas.toDataURL('image/png'));
      };
      image.onerror = () => reject(new Error('SVG failed to load'));
      image.src = 'data:image/svg+xml;base64,' + ${JSON.stringify(Buffer.from(svg).toString('base64'))};
    })
  `);
  return Buffer.from(dataUrl.split(',')[1], 'base64');
}

async function renderAll() {
  const win = new BrowserWindow({
    show: false,
    webPreferences: { offscreen: true },
  });
  await win.loadURL('data:text/html,<html><body></body></html>');
  fs.rmSync(OUT_DIR, { recursive: true, force: true });
  fs.mkdirSync(OUT_DIR, { recursive: true });
  const names = curatedIconNames();
  for (const name of names) {
    const iconNode = lucideIconNode(name);
    for (const variant of VARIANTS) {
      const svg = svgFor(iconNode, variant.color);
      for (const scale of [1, 2]) {
        const png = await rasterize(win, svg, POINT_SIZE * scale);
        const scaleSuffix = scale === 1 ? '' : `@${scale}x`;
        fs.writeFileSync(
          path.join(OUT_DIR, `${name}${variant.suffix}${scaleSuffix}.png`),
          png,
        );
      }
    }
  }
  console.log(`Rendered ${names.length} menu bar icons into ${OUT_DIR}`);
}

app.whenReady().then(async () => {
  try {
    await renderAll();
    app.exit(0);
  } catch (error) {
    console.error(error);
    app.exit(1);
  }
});
