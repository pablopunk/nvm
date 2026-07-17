import assert from 'node:assert/strict';
import test from 'node:test';
import { feedbackView } from './feedback';

test('feedback views keep context and actions in a keyboard-navigable list', () => {
  const view = feedbackView({
    id: 'uninstall-unavailable',
    title: 'Uninstall unavailable',
    message: 'This app cannot be uninstalled safely.',
    tone: 'error',
    details: [{ title: 'Play Console.app', subtitle: 'Missing app metadata' }],
  });

  assert.equal(view.type, 'list');
  assert.equal(view.selectedItemId, 'uninstall-unavailable:action:0');
  assert.deepEqual(view.items?.[0], {
    id: 'uninstall-unavailable:message',
    title: 'Uninstall unavailable',
    subtitle: 'This app cannot be uninstalled safely.',
    icon: 'circle-alert',
    appearance: { foreground: 'red' },
    disabled: true,
  });
  assert.equal(view.items?.[1].disabled, true);
  assert.equal(view.items?.[2].primaryAction?.type, 'popView');
});
