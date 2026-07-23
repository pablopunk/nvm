import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createSearchSnapshotAssembler,
  rankSearchProviderContributions,
  SEARCH_PROVIDER_RESULT_LIMIT,
  SEARCH_RESULT_LIMIT,
  type SearchAssemblyAction,
  searchActionIsVisibleInTestMode,
  searchProviderDescriptors,
} from './search-snapshot';

const PROVIDER_BASE_SCORE = 100;
const LOCAL_BASE_SCORE = 200;
const LOCAL_RESULT_COUNT = 20;
const PROVIDER_OVERFLOW_COUNT = 5;

import { normalize, scoreNormalized } from './search-utils';

interface CharacterizedAction extends SearchAssemblyAction {
  aliases?: string[];
  extensionId: string;
  id: string;
  kind: string;
  lastUsed: number;
  score: number;
  title: string;
}

function action(
  id: string,
  title: string,
  options: Partial<CharacterizedAction> = {},
): CharacterizedAction {
  return {
    extensionId: 'test.visible',
    id,
    kind: 'extension-action',
    lastUsed: 0,
    score: 10,
    title,
    ...options,
  };
}

function rankAction(item: CharacterizedAction, query: string) {
  const normalizedQuery = normalize(query);
  if (!normalizedQuery) {
    return { ...item };
  }
  const score = Math.max(
    scoreNormalized(item.title, normalizedQuery),
    ...(item.aliases || []).map((alias) =>
      scoreNormalized(alias, normalizedQuery),
    ),
  );
  return score > 0 ? { ...item, score } : null;
}

function assembler(
  query: string,
  localItems: CharacterizedAction[],
  providerKeys: string[],
) {
  return createSearchSnapshotAssembler<
    CharacterizedAction,
    CharacterizedAction
  >({
    isVisible: (item) =>
      searchActionIsVisibleInTestMode(item, {
        isSafeExtension: (extensionId) => extensionId === 'test.visible',
        progressiveRootExtensionId: 'test.progressive',
        testMode: true,
      }),
    localItems: localItems.flatMap((item) => {
      const ranked = rankAction(item, query);
      return ranked ? [ranked] : [];
    }),
    prepare: (item) => structuredClone(item),
    providerKeys,
    query,
    rankAction,
    withShortcutHint: (item) => item,
  });
}

function identity(rows: CharacterizedAction[]) {
  return rows.map(({ id, score }) => ({ id, score }));
}

const RANKING_CASES = [
  {
    name: 'empty',
    query: '',
    local: [action('local', 'Local', { score: 60 })],
    first: [action('root-a', 'Root A', { score: 50 })],
    second: [action('root-b', 'Root B', { score: 40 })],
    expected: [
      { id: 'local', score: 60 },
      { id: 'root-a', score: 50 },
      { id: 'root-b', score: 40 },
    ],
  },
  {
    name: 'fuzzy',
    query: 'stng',
    local: [action('settings', 'Settings')],
    first: [action('strong', 'Strong')],
    second: [action('ignored', 'Unrelated')],
    expected: [
      { id: 'settings', score: 20 },
      { id: 'strong', score: 20 },
    ],
  },
  {
    name: 'alias',
    query: 'preferences',
    local: [action('settings', 'Settings', { aliases: ['preferences'] })],
    first: [action('other', 'Other')],
    second: [],
    expected: [{ id: 'settings', score: 100 }],
  },
  {
    name: 'tie',
    query: '',
    local: [],
    first: [action('registered-first', 'Same', { score: 50 })],
    second: [action('registered-second', 'Same', { score: 50 })],
    expected: [
      { id: 'registered-first', score: 50 },
      { id: 'registered-second', score: 50 },
    ],
  },
];

test('selects root and query providers without crossing contribution paths', () => {
  const extensions = [
    { id: 'root', rootItems: () => [] },
    { id: 'query', searchItems: () => [] },
    {
      id: 'both',
      __filePath: '/extensions/both.ts',
      rootItems: () => [],
      searchItems: () => [],
    },
  ];

  assert.deepEqual(
    searchProviderDescriptors(extensions, '').map(({ key, kind }) => ({
      key,
      kind,
    })),
    [
      { key: '0:root', kind: 'root' },
      { key: '2:/extensions/both.ts', kind: 'root' },
    ],
  );
  assert.deepEqual(
    searchProviderDescriptors(extensions, 'settings').map(({ key, kind }) => ({
      key,
      kind,
    })),
    [
      { key: '1:query', kind: 'query' },
      { key: '2:/extensions/both.ts', kind: 'query' },
    ],
  );
});

test('applies the production test-mode action and progressive-root policy', () => {
  const options = {
    isSafeExtension: (extensionId: unknown) => extensionId === 'test.visible',
    progressiveRootExtensionId: 'test.progressive',
    testMode: true,
  };
  assert.equal(
    searchActionIsVisibleInTestMode(
      action('test', 'Test', { kind: 'test-action' }),
      options,
    ),
    true,
  );
  assert.equal(
    searchActionIsVisibleInTestMode(action('safe', 'Safe'), options),
    true,
  );
  assert.equal(
    searchActionIsVisibleInTestMode(
      action('safe-root', 'Safe root', {
        extensionId: 'test.visible',
        kind: 'extension-root-item',
      }),
      options,
    ),
    true,
  );
  assert.equal(
    searchActionIsVisibleInTestMode(
      action('root', 'Root', {
        extensionId: 'test.progressive',
        kind: 'extension-root-item',
      }),
      options,
    ),
    true,
  );
  assert.equal(
    searchActionIsVisibleInTestMode(
      action('hidden', 'Hidden', { extensionId: 'test.hidden' }),
      options,
    ),
    false,
  );
  assert.equal(
    searchActionIsVisibleInTestMode(
      action('other-root', 'Other root', { kind: 'extension-root-item' }),
      options,
    ),
    false,
  );
});

test('progressive final exactly matches one-shot for empty, fuzzy, alias, and tie cases', () => {
  for (const fixture of RANKING_CASES) {
    const keys = ['provider:first', 'provider:second'];
    const buildOneShot = assembler(fixture.query, fixture.local, keys);
    const oneShot = buildOneShot(
      new Map([
        [keys[0], fixture.first],
        [keys[1], fixture.second],
      ]),
    );
    const reverseSettled = new Map<string, CharacterizedAction[]>();
    reverseSettled.set(keys[1], fixture.second);
    reverseSettled.set(keys[0], fixture.first);
    const progressiveFinal = assembler(
      fixture.query,
      fixture.local,
      keys,
    )(reverseSettled);

    assert.deepEqual(identity(oneShot), fixture.expected, fixture.name);
    assert.deepEqual(
      identity(progressiveFinal),
      identity(oneShot),
      fixture.name,
    );
  }
});

test('preserves provider registration order for exact ties despite reverse settlement', () => {
  const keys = ['provider:first', 'provider:second'];
  const results = new Map<string, CharacterizedAction[]>([
    [keys[1], [action('second', 'Same', { score: 50 })]],
    [keys[0], [action('first', 'Same', { score: 50 })]],
  ]);

  assert.deepEqual(
    assembler('', [], keys)(results).map(({ id }) => id),
    ['first', 'second'],
  );
});

test('applies the 20-per-provider and 30-global caps with test-mode filtering', () => {
  const providerItems = Array.from(
    { length: SEARCH_PROVIDER_RESULT_LIMIT + PROVIDER_OVERFLOW_COUNT },
    (_, index) =>
      action(`provider-${index}`, `Provider ${index}`, {
        score: PROVIDER_BASE_SCORE - index,
      }),
  );
  const cappedProvider = rankSearchProviderContributions(
    providerItems,
    '',
    rankAction,
  );
  assert.equal(cappedProvider.length, SEARCH_PROVIDER_RESULT_LIMIT);

  const localItems = [
    action('hidden', 'Hidden', {
      extensionId: 'test.hidden',
      kind: 'extension-action',
      score: 1000,
    }),
    ...Array.from({ length: LOCAL_RESULT_COUNT }, (_, index) =>
      action(`local-${index}`, `Local ${index}`, {
        score: LOCAL_BASE_SCORE - index,
      }),
    ),
  ];
  const final = assembler('', localItems, ['provider'])(
    new Map([['provider', cappedProvider]]),
  );

  assert.equal(final.length, SEARCH_RESULT_LIMIT);
  assert.equal(
    final.some(({ id }) => id === 'hidden'),
    false,
  );
  assert.equal(final.filter(({ id }) => id.startsWith('provider-')).length, 10);
});
