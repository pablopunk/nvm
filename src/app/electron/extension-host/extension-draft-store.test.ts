// biome-ignore-all lint: Validation-boundary tests use inline input patterns and node:test idioms.
import assert from 'node:assert/strict';
import test from 'node:test';
import { createExtensionDraftStore } from './extension-draft-store';

test('recovers matching owner-scoped drafts and keeps owners isolated', () => {
  const drafts = createExtensionDraftStore();
  drafts.save('notes', 'note:a', 1, 'saved');

  assert.deepEqual(drafts.open('notes', 'note:a', 1, 'initial'), {
    kind: 'recovered',
    content: 'saved',
  });
  assert.deepEqual(drafts.open('other', 'note:a', 1, 'initial'), {
    kind: 'empty',
    content: 'initial',
  });
});

test('resolves a version conflict atomically and rejects replay or retargeting', () => {
  let time = 10;
  const drafts = createExtensionDraftStore({ now: () => time });
  drafts.save('notes', 'note:a', 1, 'old');
  const opened = drafts.open('notes', 'note:a', 2, 'new');
  assert.equal(opened.kind, 'conflict');
  if (opened.kind !== 'conflict') throw new Error('expected conflict');

  assert.throws(
    () =>
      drafts.resolve('notes', opened.conflict.handle, {
        type: 'draftResolution',
        key: 'note:b',
        resolution: 'migrate',
        content: 'bad',
      }),
    /key mismatch/,
  );
  assert.equal(drafts.records.get('notes\u0000note:a')?.content, 'old');

  const fresh = drafts.open('notes', 'note:a', 2, 'new');
  assert.equal(fresh.kind, 'conflict');
  if (fresh.kind !== 'conflict') throw new Error('expected conflict');
  assert.equal(
    drafts.resolve('notes', fresh.conflict.handle, {
      type: 'draftResolution',
      key: 'note:a',
      resolution: 'migrate',
      content: 'migrated',
    }).content,
    'migrated',
  );
  assert.throws(
    () =>
      drafts.resolve('notes', fresh.conflict.handle, {
        type: 'draftResolution',
        key: 'note:a',
        resolution: 'restore-old',
      }),
    /Stale/,
  );
  time += 1;
});

test('rejects malformed bounded draft inputs and purges all owner state', () => {
  const drafts = createExtensionDraftStore();
  assert.throws(() => drafts.save('notes', '', 1, 'bad'), /draft key/);
  assert.throws(() => drafts.save('notes', 'a', Number.NaN, 'bad'), /version/);
  drafts.save('notes', 'a', 1, 'one');
  drafts.save('other', 'a', 1, 'two');
  drafts.purgeOwner('notes');
  assert.equal(drafts.records.has('notes\u0000a'), false);
  assert.equal(drafts.records.has('other\u0000a'), true);
});

test('commit advances the stored version so the next open recovers', () => {
  const drafts = createExtensionDraftStore();
  drafts.save('notes', 'note:a', 1, 'unsaved');

  assert.equal(drafts.commit('notes', 'missing', 2), null);
  assert.equal(drafts.commit('notes', 'note:a', 2)?.draftVersion, 2);
  assert.deepEqual(drafts.open('notes', 'note:a', 2, 'current'), {
    kind: 'recovered',
    content: 'unsaved',
  });
  assert.throws(() => drafts.commit('notes', 'note:a', -1), /version/);
});

test('resolveByKey resolves the latest live conflict without a handle', () => {
  const drafts = createExtensionDraftStore();
  drafts.save('notes', 'note:a', 1, 'old');
  const opened = drafts.open('notes', 'note:a', 2, 'new');
  assert.equal(opened.kind, 'conflict');

  assert.throws(
    () =>
      drafts.resolveByKey('notes', 'note:b', {
        type: 'draftResolution',
        key: 'note:b',
        resolution: 'reset',
      }),
    /No live draft conflict/,
  );
  assert.equal(
    drafts.resolveByKey('notes', 'note:a', {
      type: 'draftResolution',
      key: 'note:a',
      resolution: 'restore-old',
    }).content,
    'old',
  );
  assert.deepEqual(drafts.open('notes', 'note:a', 2, 'new'), {
    kind: 'recovered',
    content: 'old',
  });
});

test('hydrate restores persisted records and skips malformed entries', () => {
  const drafts = createExtensionDraftStore();
  drafts.hydrate([
    {
      schemaVersion: 1,
      ownerExtensionId: 'notes',
      draftKey: 'note:a',
      draftVersion: 3,
      content: 'persisted',
    },
    { schemaVersion: 1, ownerExtensionId: '', draftKey: 'x' } as never,
  ]);
  assert.deepEqual(drafts.open('notes', 'note:a', 3, 'current'), {
    kind: 'recovered',
    content: 'persisted',
  });
  assert.equal(drafts.list().length, 1);
});
