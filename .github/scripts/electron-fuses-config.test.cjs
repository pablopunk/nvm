'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const { parse: parseYaml } = require('yaml');

const electronBuilderConfigPath = path.resolve(
  __dirname,
  '..',
  '..',
  'electron-builder.yml',
);

function readElectronBuilderConfig() {
  return parseYaml(fs.readFileSync(electronBuilderConfigPath, 'utf8'));
}

test('packaged Electron fuses refuse ELECTRON_RUN_AS_NODE, NODE_OPTIONS and inspect arguments', function disablesCodeInjectionFuses() {
  const fuses = readElectronBuilderConfig().electronFuses;
  assert.equal(fuses?.runAsNode, false);
  assert.equal(fuses?.enableNodeOptionsEnvironmentVariable, false);
  assert.equal(fuses?.enableNodeCliInspectArguments, false);
});
