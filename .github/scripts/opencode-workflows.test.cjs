const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const root = path.resolve(__dirname, '../..');
const opencodeWorkflow = fs.readFileSync(
  path.join(root, '.github/workflows/opencode.yml'),
  'utf8',
);

function jobIfBlock(workflow) {
  const match = workflow.match(/^ {4}if: \|\n([\s\S]*?)^ {4}runs-on:/m);
  assert.ok(match, 'opencode job must declare a multi-line if block');
  return match[1];
}

test('opencode job requires a trusted author association on top of the mention check', function commenterGate() {
  const condition = jobIfBlock(opencodeWorkflow);
  assert.match(condition, /contains\(github\.event\.comment\.body, ' \/oc'\)/);
  assert.match(
    condition,
    /\)\s*&&\s*contains\(fromJSON\('\["OWNER","MEMBER","COLLABORATOR"\]'\), github\.event\.comment\.author_association\)/,
  );
});
