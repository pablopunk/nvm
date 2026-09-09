// biome-ignore-all lint/style/noMagicNumbers: WCAG contrast uses fixed standard coefficients and thresholds.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { EXTENSION_WINDOW_BACKGROUND } from '../electron/extension-window-manager';

const paletteDirectory = path.dirname(fileURLToPath(import.meta.url));
const stylesDirectory = path.join(paletteDirectory, 'styles');
const themeCss = fs.readFileSync(
  path.join(stylesDirectory, 'theme.css'),
  'utf8',
);
const styleEntryCss = fs.readFileSync(
  path.join(paletteDirectory, 'styles.css'),
  'utf8',
);
const DARK_THEME_PATTERN =
  /@media \(prefers-color-scheme: dark\) \{([\s\S]*?)\n\}/;
const LIGHT_COLOR_SCHEME_PATTERN = /color-scheme: light;/;
const DARK_COLOR_SCHEME_PATTERN = /color-scheme: dark;/;
const RAW_COLOR_PATTERN = /#[\da-f]{3,8}\b|rgba?\(/i;
const HEX_TOKEN_PATTERN = /(--[\w-]+):\s*(#[\da-f]{6})\s*;/gi;

const requiredThemeTokens = [
  '--surface-canvas',
  '--surface-panel',
  '--surface-raised',
  '--surface-interactive',
  '--surface-selected',
  '--surface-sunken',
  '--surface-overlay',
  '--material-panel',
  '--material-raised',
  '--material-overlay',
  '--edge-subtle',
  '--edge-strong',
  '--edge-highlight',
  '--shadow-panel',
  '--shadow-raised',
  '--shadow-contact',
  '--shadow-overlay',
  '--shadow-selected',
  '--shadow-inset',
  '--text-strong',
  '--text-primary',
  '--text-secondary',
  '--text-tertiary',
  '--text-muted',
  '--accent',
  '--accent-fill',
  '--accent-contrast',
  '--accent-soft',
  '--success-text',
  '--warning-text',
  '--danger-text',
];

function hexThemeTokens(css: string) {
  return Object.fromEntries(
    [...css.matchAll(HEX_TOKEN_PATTERN)].map((match) => [match[1], match[2]]),
  );
}

function relativeLuminance(hex: string) {
  const channels = [1, 3, 5].map((offset) =>
    Number.parseInt(hex.slice(offset, offset + 2), 16),
  );
  const [red, green, blue] = channels.map((channel) => {
    const value = channel / 255;
    return value <= 0.040_45 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  });
  return red * 0.2126 + green * 0.7152 + blue * 0.0722;
}

function contrastRatio(first: string, second: string) {
  const brightest = Math.max(
    relativeLuminance(first),
    relativeLuminance(second),
  );
  const darkest = Math.min(relativeLuminance(first), relativeLuminance(second));
  return (brightest + 0.05) / (darkest + 0.05);
}

test('theme contract defines semantic tokens for light and dark appearances', () => {
  const darkTheme = themeCss.match(DARK_THEME_PATTERN)?.[1];
  assert.ok(darkTheme);
  for (const token of requiredThemeTokens) {
    assert.ok(themeCss.includes(`${token}:`), token);
    assert.ok(darkTheme.includes(`${token}:`), token);
  }
  assert.match(themeCss, LIGHT_COLOR_SCHEME_PATTERN);
  assert.match(darkTheme, DARK_COLOR_SCHEME_PATTERN);
});

test('palette style modules use theme tokens instead of raw colors', () => {
  const applicationStyleFiles = fs
    .readdirSync(stylesDirectory)
    .filter((file) => file.endsWith('.css') && file !== 'theme.css');
  for (const file of applicationStyleFiles) {
    const css = fs.readFileSync(path.join(stylesDirectory, file), 'utf8');
    assert.doesNotMatch(css, RAW_COLOR_PATTERN, file);
  }
});

test('semantic text colors meet WCAG AA on panel materials', () => {
  const darkTheme = themeCss.match(DARK_THEME_PATTERN)?.[1];
  assert.ok(darkTheme);
  const darkThemeStart = themeCss.indexOf(
    '@media (prefers-color-scheme: dark)',
  );
  assert.notEqual(darkThemeStart, -1);
  const themeTokenSets = [
    hexThemeTokens(themeCss.slice(0, darkThemeStart)),
    hexThemeTokens(darkTheme),
  ];
  const readableTokens = [
    '--text-strong',
    '--text-primary',
    '--text-secondary',
    '--text-tertiary',
    '--text-muted',
    '--link',
    '--accent',
    '--success-text',
    '--warning-text',
    '--danger-text',
    '--item-foreground-yellow',
    '--item-foreground-blue',
    '--item-foreground-purple',
    '--item-foreground-green',
    '--item-foreground-red',
    '--item-foreground-orange',
    '--item-foreground-pink',
  ];

  for (const tokens of themeTokenSets) {
    const panel = tokens['--surface-panel'];
    assert.ok(panel);
    for (const token of readableTokens) {
      const foreground = tokens[token];
      assert.ok(foreground, token);
      assert.ok(contrastRatio(foreground, panel) >= 4.5, token);
    }
    assert.ok(
      contrastRatio(tokens['--accent-contrast'], tokens['--accent-fill']) >=
        4.5,
      '--accent-contrast',
    );
  }
});

test('palette style entry imports modules in cascade order', () => {
  assert.equal(
    styleEntryCss,
    [
      '@import "./styles/theme.css";',
      '@import "./styles/base.css";',
      '@import "./styles/palette.css";',
      '@import "./styles/views.css";',
      '@import "./styles/extension-windows.css";',
      '@import "./styles/motion.css";',
      '',
    ].join('\n'),
  );
});

test('native extension-window backgrounds match renderer canvases', () => {
  const darkTheme = themeCss.match(DARK_THEME_PATTERN)?.[1];
  assert.ok(darkTheme);
  const darkThemeStart = themeCss.indexOf(
    '@media (prefers-color-scheme: dark)',
  );
  const lightTokens = hexThemeTokens(themeCss.slice(0, darkThemeStart));
  const darkTokens = hexThemeTokens(darkTheme);

  assert.equal(
    lightTokens['--surface-canvas'],
    EXTENSION_WINDOW_BACKGROUND.light,
  );
  assert.equal(
    darkTokens['--surface-canvas'],
    EXTENSION_WINDOW_BACKGROUND.dark,
  );
});
