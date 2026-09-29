import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  formFieldMatches,
  formValueLabel,
  invalidFormFields,
} from './form-fields';

test('form field rows show selected choices and conceal secrets', () => {
  assert.equal(
    formValueLabel(
      {
        id: 'device',
        type: 'dropdown',
        options: [{ title: 'Built-in microphone', value: 'default' }],
      },
      'default',
    ),
    'Built-in microphone',
  );
  assert.equal(
    formValueLabel({ id: 'password', type: 'password' }, 'secret'),
    '••••••',
  );
  assert.equal(
    formValueLabel({ id: 'tags', type: 'multiselect' }, ['a', 'b']),
    '2 selected',
  );
});

test('form field search includes the current value and help text', () => {
  const field = {
    id: 'device',
    label: 'Microphone',
    description: 'Speech input',
  };
  assert.equal(formFieldMatches(field, 'Yeti', 'yeti'), true);
  assert.equal(formFieldMatches(field, 'Yeti', 'speech'), true);
  assert.equal(formFieldMatches(field, 'Yeti', 'output'), false);
});

test('form validation reports required fields and ignores disabled fields', () => {
  assert.deepEqual(
    invalidFormFields(
      [
        { id: 'name', label: 'Name', required: true },
        { id: 'consent', type: 'checkbox', required: true },
        { id: 'files', type: 'files', required: true },
        { id: 'unavailable', required: true, disabled: true },
      ],
      { name: ' ', consent: false, files: [], unavailable: '' },
    ),
    {
      name: 'Name is required',
      consent: 'consent is required',
      files: 'files is required',
    },
  );
});
