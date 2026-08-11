import assert from 'node:assert/strict';
import test from 'node:test';
import {
  DESIGN_TOKEN_DEFAULTS,
  resolveDesignTokens,
  validateDesignTokenOverrides,
} from './design-tokens';

test('design token overrides retain only changed allowlisted values', () => {
  assert.deepEqual(
    validateDesignTokenOverrides({
      '--radius-lg': '20px',
      '--accent': DESIGN_TOKEN_DEFAULTS['--accent'],
    }),
    { '--radius-lg': '20px' },
  );
  assert.equal(
    resolveDesignTokens({ '--radius-lg': '20px' })['--radius-lg'],
    '20px',
  );
  assert.equal(resolveDesignTokens({})['--accent'], '#ffd84d');
});

test('design token overrides reject unknown keys and CSS injection', () => {
  for (const input of [
    { '--unknown': '10px' },
    { '--radius-lg': '10px; color: red' },
    { '--accent': 'url(https://example.com/a)' },
    { '--accent': 'red' },
    { '--font-family-ui': 'system-ui; background:red' },
    { '--accent': 'a'.repeat(241) },
  ]) {
    assert.throws(() => validateDesignTokenOverrides(input));
  }
});

test('design token overrides accept constrained lengths, colors, and fonts', () => {
  assert.deepEqual(
    validateDesignTokenOverrides({
      '--palette-stack-gap': '12.5px',
      '--text-primary': 'rgba(240, 240, 245, 0.95)',
      '--font-family-ui': 'Inter, system-ui, sans-serif',
    }),
    {
      '--palette-stack-gap': '12.5px',
      '--text-primary': 'rgba(240, 240, 245, 0.95)',
      '--font-family-ui': 'Inter, system-ui, sans-serif',
    },
  );
});

test('resolved design tokens are structured-clone safe', () => {
  const state = {
    enabled: true,
    defaults: { ...DESIGN_TOKEN_DEFAULTS },
    overrides: validateDesignTokenOverrides({ '--radius-sm': '10px' }),
    values: resolveDesignTokens({ '--radius-sm': '10px' }),
  };
  assert.deepEqual(structuredClone(state), state);
});
