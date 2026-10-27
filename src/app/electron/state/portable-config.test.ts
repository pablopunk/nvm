import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyPortableChanges,
  applyPortableConfigToRecord,
  diffPortableConfig,
  portableConfigFromRecord,
  recordWithoutPortableFields,
} from './portable-config-projection';
import {
  createDefaultPortableConfig,
  parsePortableConfigText,
} from './portable-config-schema';

const KINDS = { paletteHotkey: 'shortcut', errorReporting: 'boolean' } as const;

function validDocument(overrides: Record<string, unknown> = {}) {
  return JSON.stringify({
    version: 1,
    settings: { paletteHotkey: 'Command+Alt+K' },
    aliases: { 'default:calculator': ['math'] },
    shortcuts: {
      'extension-action:demo:open': 'Command+Shift+O',
      'extension-action:demo:gone': null,
    },
    jobs: { enabled: { 'demo.sync': false } },
    ai: { defaultModel: 'smart' },
    ...overrides,
  });
}

test('parses a valid document and ignores unknown setting ids', () => {
  const text = validDocument({
    settings: { paletteHotkey: 'Command+Alt+K', legacyThing: 1 },
  });
  const parsed = parsePortableConfigText(text, KINDS);
  assert.equal(parsed.ok, true);
  if (!parsed.ok) return;
  assert.deepEqual(parsed.config.settings, { paletteHotkey: 'Command+Alt+K' });
  assert.equal(parsed.config.shortcuts['extension-action:demo:gone'], null);
  assert.equal(parsed.config.ai.defaultModel, 'smart');
});

test('rejects unsupported versions, malformed JSON, and wrong value types', () => {
  assert.equal(
    parsePortableConfigText(validDocument({ version: 2 }), KINDS).ok,
    false,
  );
  assert.equal(parsePortableConfigText('{ nope', KINDS).ok, false);
  assert.equal(parsePortableConfigText('[]', KINDS).ok, false);
  assert.equal(
    parsePortableConfigText(
      validDocument({ settings: { errorReporting: 'yes' } }),
      KINDS,
    ).ok,
    false,
  );
  assert.equal(
    parsePortableConfigText(validDocument({ aliases: { x: [''] } }), KINDS).ok,
    false,
  );
  assert.equal(
    parsePortableConfigText(
      validDocument({ jobs: { enabled: { job: 'on' } } }),
      KINDS,
    ).ok,
    false,
  );
});

test('changes keep unknown top-level and nested fields when written back', () => {
  const raw = JSON.parse(validDocument({ futureSection: { keep: true } }));
  raw.aliases.extra = ['kept'];
  const next = applyPortableChanges(raw, [
    {
      path: ['settings', 'paletteHotkey'],
      removed: false,
      value: 'Command+Alt+J',
    },
    { path: ['aliases', 'default:calculator'], removed: true },
  ]);
  assert.deepEqual(next.futureSection, { keep: true });
  assert.deepEqual(next.aliases, { extra: ['kept'] });
  assert.equal(next.settings.paletteHotkey, 'Command+Alt+J');
});

test('record projection excludes private fields and maps disabled declared shortcuts to null', () => {
  const record: Record<string, any> = {
    settings: { paletteHotkey: 'Command+Alt+K', unknownSetting: true },
    aliases: { a: ['x'], empty: [] },
    shortcuts: { 'extension-action:demo:open': 'Command+Shift+O' },
    removedShortcuts: { 'extension-action:demo:gone': true },
    jobSettings: { enabled: { 'demo.sync': false } },
    aiChatDefaultModel: 'smart',
    shortcutActions: { private: { aiChatId: 'chat-1' } },
    recents: { private: 1 },
  };
  const config = portableConfigFromRecord(record, KINDS);
  assert.deepEqual(config.settings, { paletteHotkey: 'Command+Alt+K' });
  assert.deepEqual(config.aliases, { a: ['x'] });
  assert.deepEqual(config.shortcuts, {
    'extension-action:demo:open': 'Command+Shift+O',
    'extension-action:demo:gone': null,
  });
  const privateRecord = recordWithoutPortableFields(record);
  assert.equal('settings' in privateRecord, false);
  assert.deepEqual(privateRecord.shortcutActions, record.shortcutActions);
  assert.deepEqual(privateRecord.recents, record.recents);
});

test('applying a config to a record round-trips through the projection', () => {
  const config = parsePortableConfigText(validDocument(), KINDS);
  assert.equal(config.ok, true);
  if (!config.ok) return;
  const record: Record<string, any> = {};
  applyPortableConfigToRecord(record, config.config);
  assert.equal(record.removedShortcuts['extension-action:demo:gone'], true);
  assert.deepEqual(portableConfigFromRecord(record, KINDS), config.config);
});

test('diff lists only changed keys and defaults compare equal', () => {
  const base = createDefaultPortableConfig();
  assert.deepEqual(diffPortableConfig(base, createDefaultPortableConfig()), []);
  const next = {
    ...base,
    aliases: { x: ['y'] },
    ai: { defaultModel: 'smart' as const },
  };
  assert.deepEqual(
    diffPortableConfig(base, next).map((change) => change.path.join('.')),
    ['aliases.x', 'ai.defaultModel'],
  );
});
