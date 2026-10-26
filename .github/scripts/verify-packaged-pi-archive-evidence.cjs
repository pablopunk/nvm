const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

function verifyPackagedPiArchiveEvidence(
  directory,
  expectedSha,
  expectedPlatform,
) {
  const evidencePath = path.join(directory, 'pi-archive-node-evidence.json');
  if (!fs.existsSync(evidencePath))
    throw new Error(`Packaged Pi evidence is missing: ${evidencePath}`);
  const evidence = JSON.parse(fs.readFileSync(evidencePath, 'utf8'));
  const manifest = JSON.parse(
    fs.readFileSync(path.resolve(__dirname, '../../package.json'), 'utf8'),
  );
  assert.equal(evidence.commitSha, expectedSha);
  assert.equal(
    evidence.runtimeBoundary,
    'stock-node-session-from-extracted-app-asar',
  );
  assert.equal(evidence.runtime, 'host-node');
  assert.equal(evidence.electronRuntime, false);
  assert.equal(evidence.adapterIntegration, false);
  assert.deepEqual(evidence.loadedPiPackages, ['pi-coding-agent', 'pi-ai']);
  assert.equal(evidence.fixtureSession, 'success');
  assert.equal(evidence.provider, 'loopback-openai-completions');
  assert.equal(evidence.providerCalls, 1);
  assert.match(
    evidence.loadedWebExtensionPath || '',
    /^node_modules\/pi-web-access\//,
    'Pi must load the bundled web extension from the app archive',
  );
  assert.deepEqual(evidence.packageVersions, {
    'pi-coding-agent': manifest.dependencies['@earendil-works/pi-coding-agent'],
    'pi-ai': manifest.dependencies['@earendil-works/pi-ai'],
    'pi-web-access': manifest.dependencies['pi-web-access'],
  });
  for (const packageName of ['pi-coding-agent', 'pi-ai', 'pi-web-access'])
    assert.match(
      evidence.resolvedPackageEntries?.[packageName] || '',
      /^node_modules\//,
      `${packageName} must resolve from the extracted app archive`,
    );
  if (expectedPlatform) assert.equal(evidence.architecture, expectedPlatform);
  return evidence;
}

if (require.main === module) {
  try {
    const evidence = verifyPackagedPiArchiveEvidence(
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

module.exports = { verifyPackagedPiArchiveEvidence };
