const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

function verifyPackagedPiEvidence(directory, expectedSha, expectedPlatform) {
  const evidencePath = path.join(directory, 'pi-session-evidence.json');
  if (!fs.existsSync(evidencePath))
    throw new Error(`Packaged Pi evidence is missing: ${evidencePath}`);
  const evidence = JSON.parse(fs.readFileSync(evidencePath, 'utf8'));
  const manifest = JSON.parse(fs.readFileSync('package.json', 'utf8'));
  assert.equal(evidence.commitSha, expectedSha);
  assert.equal(evidence.fixtureSession, 'success');
  assert.equal(evidence.provider, 'loopback-openai-completions');
  assert.equal(evidence.providerCalls, 1);
  assert.equal(
    evidence.packagedImports['pi-coding-agent'],
    manifest.dependencies['@earendil-works/pi-coding-agent'],
  );
  assert.equal(
    evidence.packagedImports['pi-ai'],
    manifest.dependencies['@earendil-works/pi-ai'],
  );
  assert.equal(
    evidence.packagedImports['pi-web-access'],
    manifest.dependencies['pi-web-access'],
  );
  if (expectedPlatform) assert.equal(evidence.architecture, expectedPlatform);
  return evidence;
}

if (require.main === module) {
  try {
    const evidence = verifyPackagedPiEvidence(
      path.resolve(process.argv[2]),
      process.argv[3],
      process.argv[4],
    );
    console.log(JSON.stringify(evidence));
  } catch (error) {
    console.error(error);
    process.exitCode = 1;
  }
}

module.exports = { verifyPackagedPiEvidence };
