import assert from 'node:assert/strict';
import test from 'node:test';
import { renderDocMarkup, summarizeDoc } from './docs-markup';

test('escapes HTML from doc comments', () => {
  const html = renderDocMarkup('Use <script>alert(1)</script> & `<b>`');
  assert.equal(html.includes('<script>'), false);
  assert.match(html, /&lt;script&gt;/);
  assert.match(html, /<code>&lt;b&gt;<\/code>/);
});

test('renders paragraphs, lists, links, and fenced code', () => {
  const html = renderDocMarkup(
    'First paragraph with {@link Thing}.\n\n- one\n- two\n\n```ts\nconst a = 1 < 2;\n```',
  );
  assert.match(html, /<p>First paragraph with <code>Thing<\/code>\.<\/p>/);
  assert.match(html, /<ul><li>one<\/li><li>two<\/li><\/ul>/);
  assert.match(html, /<pre><code>const a = 1 &lt; 2;<\/code><\/pre>/);
});

test('summarizes a doc comment to its first sentence', () => {
  assert.equal(
    summarizeDoc('Declarative host UI. Nevermind owns rendering.\n\nMore.'),
    'Declarative host UI.',
  );
  assert.equal(summarizeDoc('No terminal punctuation'), 'No terminal punctuation');
});
