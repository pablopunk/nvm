const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const repository = path.resolve(__dirname, '../..');

test('overflow errors cannot reset the active builder session before Pi settles', function noEarlyOverflowReset() {
  const handler = fs.readFileSync(
    path.join(repository, 'src/app/electron/ai/nevermind-ai-event-handler.ts'),
    'utf8',
  );
  assert.doesNotMatch(handler, /activeBuilderSessionIds/);
  assert.doesNotMatch(handler, /getNevermindAi/);
  assert.doesNotMatch(handler, /\.reset\(chatId\)/);
});

test('Pi diagnostics never persist raw provider tool-call text', function noRawProviderText() {
  const ai = fs.readFileSync(
    path.join(repository, 'src/app/electron/ai/ai.ts'),
    'utf8',
  );
  assert.match(ai, /ai\.provider\.tool_call_text\.detected/);
  assert.doesNotMatch(ai, /logger\.warn\([\s\S]{0,300}?\bdelta\s*,/);
});
