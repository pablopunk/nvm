import assert from 'node:assert/strict';
import test from 'node:test';
import { renderToString } from 'react-dom/server';
import { iconForAction, iconForItem } from './command-icons';
import type { CommandAction } from './model';

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

test('item images let their draggable result row own the drag gesture', () => {
  const html = renderToString(
    iconForItem({
      id: 'app',
      title: 'App',
      image: 'file:///Applications/App.app/Contents/Resources/App.png',
    } as any),
  );

  assert.match(html, /draggable="false"/);
});

function actionIconClass(action: Partial<CommandAction>) {
  const html = renderToString(
    iconForAction({
      title: '',
      type: 'runExtensionAction',
      ...action,
    } as CommandAction),
  );
  return html
    .match(/lucide-([a-z-]+)/g)
    ?.find((name) => name !== 'lucide-icon');
}

test('actions without an icon get an intent icon instead of a web globe', () => {
  assert.equal(actionIconClass({ title: 'Add Task' }), 'lucide-plus');
  assert.equal(
    actionIconClass({ title: 'Continue to Tools' }),
    'lucide-arrow-right',
  );
  assert.equal(
    actionIconClass({ type: 'openUrl', title: 'Open Docs' }),
    'lucide-globe',
  );
  assert.equal(
    actionIconClass({ type: 'pushView', title: 'Details' }),
    'lucide-chevron-right',
  );
  assert.equal(
    actionIconClass({ icon: 'calendar', title: 'Add Event' }),
    'lucide-calendar',
  );
});
