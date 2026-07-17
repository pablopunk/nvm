'use strict';

const fs = require('node:fs');
const path = require('node:path');

const root = path.resolve(__dirname, '..');
const apiPath = path.join(root, 'src/resources/nevermind-extension-api.d.ts');
const trustDocPath = path.join(root, 'src/docs/extension-trust-model.md');
const disclosurePath = path.join(
  root,
  'src/electron/extension-capabilities.ts',
);
const trustDoc = fs.readFileSync(trustDocPath, 'utf8');
const disclosure = fs.readFileSync(disclosurePath, 'utf8');

const forbiddenEnforcementCopy = [
  /requires? (?:the )?`[^`]+` permission/i,
  /requires? [a-z.]+ permission/i,
  /present only with (?:the )?`[^`]+` permission/i,
  /optional namespaces require matching top-level permissions/i,
  /attachments require `[^`]+`/i,
  /permission (?:is )?(?:missing|unavailable|not available)/i,
];
const extensionContractCopyFiles = [
  apiPath,
  path.join(root, 'src/electron/ai.ts'),
  path.join(root, 'src/fixtures/ui-fixtures.ts'),
];
for (const file of extensionContractCopyFiles) {
  const source = fs.readFileSync(file, 'utf8');
  for (const pattern of forbiddenEnforcementCopy) {
    if (pattern.test(source)) {
      console.error(
        `Misleading extension enforcement copy in ${path.relative(root, file)} matches ${pattern}`,
      );
      process.exit(1);
    }
  }
}

for (const phrase of [
  'trusted code',
  'full access to the user’s computer',
  'review metadata',
  'not a sandbox',
  'legacy `permissions` manifest field',
]) {
  if (!trustDoc.includes(phrase)) {
    console.error(`Extension trust documentation is missing: ${phrase}`);
    process.exit(1);
  }
}

if (
  !disclosure.includes('full access to your computer when enabled') ||
  !disclosure.includes('not security restrictions')
) {
  console.error('Extension review disclosure drifted from the trust model');
  process.exit(1);
}

const builtInFiles = [
  ...fs
    .readdirSync(path.join(root, 'src/electron/extensions'))
    .filter((name) => name.endsWith('.ts') && name !== 'extensions.ts')
    .map((name) => path.join(root, 'src/electron/extensions', name)),
  path.join(root, 'src/electron/clipboard-history.ts'),
  path.join(root, 'src/fixtures/ui-fixtures.ts'),
];
for (const file of builtInFiles) {
  if (/\bpermissions\s*:/.test(fs.readFileSync(file, 'utf8'))) {
    console.error(
      `Built-in extension still declares legacy permissions: ${path.relative(root, file)}`,
    );
    process.exit(1);
  }
}

console.log('Extension trust copy checks passed');
