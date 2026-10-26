const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const root = path.resolve(__dirname, '../..');
const updatesWorkflow = fs.readFileSync(
  path.join(root, '.github/workflows/pi-updates.yml'),
  'utf8',
);
const ciWorkflow = fs.readFileSync(
  path.join(root, '.github/workflows/ci.yml'),
  'utf8',
);

function jobSection(workflow, jobName, nextJobName) {
  const start = workflow.indexOf(`  ${jobName}:`);
  assert.notEqual(start, -1, `missing workflow job ${jobName}`);
  const end = workflow.indexOf(`  ${nextJobName}:`, start + 1);
  return workflow.slice(start, end < 0 ? undefined : end);
}

test('weekly and manual Pi update workflow uses a fixed cadence and non-overlapping runs', function scheduleAndSerialization() {
  assert.match(updatesWorkflow, /cron: '23 7 \* \* 1'/);
  assert.match(updatesWorkflow, /workflow_dispatch:/);
  assert.match(
    updatesWorkflow,
    /group: weekly-pi-sdk-update\s+cancel-in-progress: false/,
  );
  assert.doesNotMatch(
    updatesWorkflow,
    /pull_request_target|automerge|enablePullRequestAutoMerge/i,
  );
});

test('resolver job has read-only checkout and no App, PAT, or write credential', function unprivilegedResolver() {
  const resolver = jobSection(
    updatesWorkflow,
    'resolve-candidate',
    'publish-pr',
  );
  assert.match(resolver, /persist-credentials: false/);
  assert.match(resolver, /pnpm install --frozen-lockfile/);
  assert.match(resolver, /pnpm install --no-frozen-lockfile/);
  assert.match(resolver, /resolve-pi-update\.cjs/);
  assert.match(resolver, /upload-artifact@v4/);
  assert.doesNotMatch(
    resolver,
    /secrets\.|GH_APP_TOKEN|GH_PAT|permission-contents: write|permission-pull-requests: write/,
  );
  assert.match(updatesWorkflow, /permissions:\s+contents: read/);
});

test('publisher receives only package data, validates provenance, and mints a scoped immutable App token', function trustedPublisher() {
  const publisher = jobSection(updatesWorkflow, 'publish-pr', undefined);
  assert.match(
    publisher,
    /ref: \$\{\{ github\.event\.repository\.default_branch \}\}/,
  );
  assert.match(publisher, /download-artifact@v4/);
  assert.match(publisher, /publish-pi-update\.cjs --validate/);
  assert.match(publisher, /PI_UPDATE_APP_ID/);
  assert.match(publisher, /PI_UPDATE_APP_PRIVATE_KEY/);
  assert.match(
    publisher,
    /actions\/create-github-app-token@64ab729a6294000f5aa4a4bc0b73d1f32bc45510/,
  );
  assert.match(publisher, /permission-contents: write/);
  assert.match(publisher, /permission-pull-requests: write/);
  assert.match(publisher, /GITHUB_TOKEN: \$\{\{ github\.token \}\}/);
  assert.match(publisher, /contents: read\s+pull-requests: read/);
  assert.doesNotMatch(
    publisher,
    /checkout[^\n]*automation\/pi-update|GH_PAT|PULL_REQUEST_TARGET/i,
  );
  assert.match(
    updatesWorkflow,
    /PI update publishing is blocked: configure PI_UPDATE_APP_ID and PI_UPDATE_APP_PRIVATE_KEY/,
  );
});

test('candidate publisher applies data through GitHub APIs without executing artifact code', function dataOnlyPublisher() {
  const publisherScript = fs.readFileSync(
    path.join(root, '.github/scripts/publish-pi-update.cjs'),
    'utf8',
  );
  assert.match(publisherScript, /execFileSync\(\s*'git'/);
  assert.match(publisherScript, /'merge-file'/);
  assert.doesNotMatch(publisherScript, /spawn\(|exec\(|eval\(/);
  assert.match(publisherScript, /PACKAGE_FILES/);
  assert.match(publisherScript, /metadata\.runId/);
  assert.match(publisherScript, /metadata\.baseSha/);
  assert.match(publisherScript, /closedUnmergedTupleExists/);
  assert.match(publisherScript, /fingerprint/);
  assert.match(publisherScript, /generatedCommitSha/);
  assert.match(publisherScript, /mergeGeneratedFileWithCurrentBase/);
  assert.match(
    publisherScript,
    /new non-registry package resolution|validateLockfile/,
  );
});

test('every relevant PR gets one same-run fail-closed gate and three OS package jobs', function requiredCiBoundary() {
  assert.equal(
    (ciWorkflow.match(/name: Pi integration compatibility/g) || []).length,
    1,
  );
  const gate = jobSection(
    ciWorkflow,
    'pi-integration-compatibility',
    'windows-first-run',
  );
  assert.match(
    gate,
    /if: always\(\) && \(github\.event_name == 'pull_request' \|\| github\.event_name == 'merge_group'\)/,
  );
  for (const job of [
    'pi-change-classification',
    'check',
    'pi-integration-tests',
    'electron-smoke',
    'packaged-dependency-contracts',
    'packaged-diagnostics',
    'pi-packaged-macos',
  ])
    assert.ok(gate.includes(`- ${job}`), `gate must depend on ${job}`);
  assert.match(ciWorkflow, /NVM_TEST_PI_PACKAGED: '1'/g);
  assert.match(ciWorkflow, /--mac dir/);
  assert.match(ciWorkflow, /CSC_IDENTITY_AUTO_DISCOVERY: 'false'/);
  assert.match(ciWorkflow, /verify-packaged-pi-evidence\.cjs/);
  assert.match(ciWorkflow, /verify-pi-integration-evidence\.cjs/);
});
