import assert from 'node:assert/strict';
import { test } from 'node:test';
import { formKeyboardActionForEvent } from './form-keyboard';

test('form keyboard actions reserve submit and retain host shortcuts', () => {
  assert.equal(
    formKeyboardActionForEvent({ key: 'Enter', metaKey: true }),
    'submit',
  );
  assert.equal(formKeyboardActionForEvent({ key: 'y', metaKey: true }), 'host');
  assert.equal(formKeyboardActionForEvent({ key: 'k', metaKey: true }), 'host');
});

test('form keyboard actions advance text inputs without consuming textareas', () => {
  assert.equal(
    formKeyboardActionForEvent({ key: 'Enter', targetTag: 'INPUT' }),
    'advance',
  );
  assert.equal(
    formKeyboardActionForEvent({
      key: 'Enter',
      targetTag: 'INPUT',
      inputType: 'checkbox',
    }),
    'toggle',
  );
  assert.equal(
    formKeyboardActionForEvent({ key: 'Enter', targetTag: 'TEXTAREA' }),
    'field',
  );
});
