import assert from 'node:assert/strict';
import test from 'node:test';
import {
  readWebviewThemeValues,
  webviewThemeStyle,
  withWebviewTheme,
} from './webview-theme';

const values = { accent: '#f1cf54', 'radius-md': '12px' };

test('tokens become --nvm-* variables without styling elements', () => {
  const style = webviewThemeStyle(values);
  assert.match(style, /--nvm-accent:#f1cf54;/);
  assert.match(style, /--nvm-radius-md:12px;/);
  assert.doesNotMatch(style, /body|div|button/);
});

test('the theme lands inside <head> so the doctype keeps standards mode', () => {
  const html = '<!doctype html><html><head><title>x</title></head></html>';
  const themed = withWebviewTheme(html, values);
  assert.ok(themed.startsWith('<!doctype html><html><head><style'));
});

test('fragments without a head get the theme prepended', () => {
  assert.ok(
    withWebviewTheme('<div>hi</div>', values).endsWith('<div>hi</div>'),
  );
});

test('token values cannot break out of the style block', () => {
  assert.doesNotMatch(
    webviewThemeStyle({ accent: 'red;}</style><script>' }),
    /<\/style><script>/,
  );
});

test('computed palette values are read for every token', () => {
  const read = readWebviewThemeValues({
    fontFamily: 'Inter',
    getPropertyValue: (name: string) => (name === '--accent' ? ' #fff ' : ''),
  });
  assert.equal(read.accent, '#fff');
  assert.equal(read['font-sans'], 'Inter');
});

test('without theme values the html is left untouched', () => {
  assert.equal(withWebviewTheme('<p>x</p>', {}), '<p>x</p>');
});
