import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { MENUBAR_ICON_NAMES } from './menubar-icons';

const resourcesDir = path.join(import.meta.dirname, '..', 'resources');

function declaredMenuBarIcons() {
  const declaration = fs.readFileSync(
    path.join(resourcesDir, 'nevermind-extension-api.d.ts'),
    'utf8',
  );
  const union = declaration.match(
    /export type ExtensionMenuBarIcon =([\s\S]*?);/,
  );
  assert.ok(union, 'ExtensionMenuBarIcon is declared');
  return [...union[1].matchAll(/'([a-z0-9-]+)'/g)].map((match) => match[1]);
}

test('the extension API icon union matches the curated set', () => {
  assert.deepEqual(declaredMenuBarIcons(), [...MENUBAR_ICON_NAMES]);
});

test('every curated icon ships light, dark and @2x assets', () => {
  const iconsDir = path.join(resourcesDir, 'menubar-icons');
  const missing = MENUBAR_ICON_NAMES.flatMap((name) =>
    [
      `${name}.png`,
      `${name}@2x.png`,
      `${name}-white.png`,
      `${name}-white@2x.png`,
    ].filter((file) => !fs.existsSync(path.join(iconsDir, file))),
  );
  assert.deepEqual(missing, []);
});
