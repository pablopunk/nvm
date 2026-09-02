'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const test = require('node:test');

const workflow = fs.readFileSync(
  '.github/workflows/database-migration.yml',
  'utf8',
);
const vercelConfig = fs.readFileSync('src/backend/vercel.json', 'utf8');

test('production deploys the exact successful main push after migration', () => {
  assert.match(workflow, /workflow_run:\n    workflows: \[CI\]/);
  assert.match(workflow, /branches: \[main\]/);
  assert.match(workflow, /workflow_run\.conclusion == 'success'/);
  assert.match(workflow, /workflow_run\.event == 'push'/);
  assert.match(workflow, /github\.event\.workflow_run\.head_sha/);
  assert.match(workflow, /NVM_PRODUCTION_MIGRATION_APPROVED:.*'true'/);
  assert.match(workflow, /pnpm -C src\/backend db:migrate/);
  assert.match(workflow, /vercel@latest deploy --prod/);
});

test('the migration workflow remains the only deployment owner', () => {
  assert.doesNotMatch(workflow, /working-directory: src\/backend/);
  assert.equal(JSON.parse(vercelConfig).git.deploymentEnabled, false);
});
