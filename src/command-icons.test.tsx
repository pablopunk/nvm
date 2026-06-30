import assert from 'node:assert/strict';
import test from 'node:test';
import { renderToString } from 'react-dom/server';
import { iconForItem } from './command-icons';

test('extension item icons fall back instead of rendering unsafe lucide exports', () => {
  const html = renderToString(
    iconForItem({
      id: 'bad-icon',
      title: 'Bad icon',
      icon: 'icon',
    } as any),
  );

  assert.match(html, /<svg/);
  assert.match(html, /lucide/);
});

test('extension item icons render known lucide aliases', () => {
  const html = renderToString(
    iconForItem({
      id: 'trash',
      title: 'Trash',
      icon: 'trash-2',
    } as any),
  );

  assert.match(html, /<svg/);
  assert.match(html, /lucide/);
});
