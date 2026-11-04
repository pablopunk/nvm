import assert from 'node:assert/strict';
import test from 'node:test';
import {
  MATCH_THRESHOLD,
  semanticCharacterMatches,
} from './emoji-symbols-semantic';

type Question = {
  id: string;
  type: string;
  question: string;
  options?: string[];
};

function predicate(probability: number) {
  return { type: 'predicate', value: probability >= 0.5, probability };
}

/** Plays the three funnel steps: categories, recall by choice, then per-candidate verification. */
function fakeAi(cutenessByGlyph: Record<string, number>) {
  const calls: Question[][] = [];
  return {
    calls,
    async decide(options: { input: string; questions: Question[] }) {
      calls.push(options.questions);
      const answers: Record<string, unknown> = {};
      for (const question of options.questions) {
        if (question.id.startsWith('group_'))
          answers[question.id] = predicate(
            /"(Smileys & Emotion|Animals & Nature)"/.test(question.question)
              ? 0.9
              : 0.05,
          );
        else if (question.id.startsWith('recall_'))
          answers[question.id] = {
            type: 'choice',
            value: 'none of these',
            probabilities: Object.fromEntries(
              (question.options ?? []).map((option) => [
                option,
                option === 'none of these'
                  ? 0.5
                  : (cutenessByGlyph[option.split(' ')[0]] ?? 0) / 4,
              ]),
            ),
          };
        else {
          const glyph = question.question.split(' ')[1];
          answers[question.id] = predicate(cutenessByGlyph[glyph] ?? 0);
        }
      }
      return { id: 'd', model: 'test', answers, usage: {} };
    },
  };
}

test('narrows by category, shortlists by choice, and keeps verified matches best first', async () => {
  const ai = fakeAi({ '🥺': 0.7, '🐱': 0.95, '🐯': 0.4, '🐶': 0.6 });
  const matches = await semanticCharacterMatches(ai as any, 'all', 'Cute ');

  assert.deepEqual(
    matches.map((record) => record.glyph),
    ['🐱', '🥺', '🐶'],
  );
  assert.equal(ai.calls.length, 3);
  assert.ok(ai.calls[1].length <= 16);
  assert.ok(ai.calls[1].every((question) => question.options!.length <= 32));
  assert.ok(ai.calls[2].length <= 16);
  assert.ok(MATCH_THRESHOLD > 0.4);
});

test('a chosen category skips the category step', async () => {
  const ai = fakeAi({ '🐱': 0.95 });
  const matches = await semanticCharacterMatches(
    ai as any,
    'Animals & Nature',
    'adorable pet',
  );

  assert.deepEqual(
    matches.map((record) => record.glyph),
    ['🐱'],
  );
  assert.equal(ai.calls.length, 2);
  assert.ok(ai.calls[0][0].id.startsWith('recall_'));
});

test('categories matched only by name never ask the model', async () => {
  const ai = fakeAi({});
  assert.deepEqual(
    await semanticCharacterMatches(ai as any, 'Flags', 'france'),
    [],
  );
  assert.equal(ai.calls.length, 0);
});

test('short text never asks the model', async () => {
  const ai = fakeAi({});
  assert.deepEqual(await semanticCharacterMatches(ai as any, 'all', 'ox'), []);
  assert.equal(ai.calls.length, 0);
});

test('repeating a search is answered from memory', async () => {
  const ai = fakeAi({ '🐱': 0.95 });
  await semanticCharacterMatches(ai as any, 'all', 'kitten time');
  const callsAfterFirst = ai.calls.length;
  await semanticCharacterMatches(ai as any, 'all', 'Kitten Time');
  assert.equal(ai.calls.length, callsAfterFirst);
});

test('stops after the category step when no category is likely', async () => {
  const ai = {
    calls: 0,
    async decide(options: { questions: Question[] }) {
      this.calls += 1;
      return {
        id: 'd',
        model: 'test',
        usage: {},
        answers: Object.fromEntries(
          options.questions.map((question) => [question.id, predicate(0.01)]),
        ),
      };
    },
  };
  assert.deepEqual(
    await semanticCharacterMatches(ai as any, 'all', 'quarterly invoices'),
    [],
  );
  assert.equal(ai.calls, 1);
});
