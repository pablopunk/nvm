const crypto = require('node:crypto');
const fs = require('node:fs/promises');
const path = require('node:path');

const [metadataPath, artifactDirectory] = process.argv.slice(2);
if (!(metadataPath && artifactDirectory))
  throw new Error(
    'Usage: validate-linux-updater-metadata <metadata.yml> <artifact-directory>',
  );

function yamlValue(line) {
  return line.trim().replace(/^['"]|['"]$/g, '');
}

function requiredMatch(body, expression, description) {
  const match = body.match(expression);
  if (!match) throw new Error(`Missing ${description} in ${metadataPath}`);
  return yamlValue(match[1]);
}

async function main() {
  const metadata = await fs.readFile(metadataPath, 'utf8');
  const artifactNames = await fs.readdir(artifactDirectory);
  const appImageNames = artifactNames.filter((name) =>
    name.endsWith('.AppImage'),
  );
  const debNames = artifactNames.filter((name) => name.endsWith('.deb'));
  if (appImageNames.length !== 1)
    throw new Error(
      `Expected exactly one AppImage in ${artifactDirectory}, found ${appImageNames.length}`,
    );
  if (debNames.length !== 1)
    throw new Error(
      `Expected exactly one deb in ${artifactDirectory}, found ${debNames.length}`,
    );

  const appImageName = appImageNames[0];
  const blockmapPath = path.join(artifactDirectory, `${appImageName}.blockmap`);
  const blockmap = await fs.stat(blockmapPath).catch(() => null);
  if (!(blockmap?.isFile() && blockmap.size > 0))
    throw new Error(`Missing non-empty blockmap for ${appImageName}`);
  const pathValue = requiredMatch(
    metadata,
    /^path:\s*(.+)$/m,
    'top-level path',
  );
  const sha512 = requiredMatch(
    metadata,
    /^sha512:\s*(.+)$/m,
    'top-level sha512',
  );
  const fileEntry = metadata.match(
    new RegExp(
      `^\\s*- url:\\s*${appImageName.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\$&')}\\s*\\n\\s*sha512:\\s*(.+)$`,
      'm',
    ),
  );
  if (!fileEntry)
    throw new Error(
      `Missing files entry for ${appImageName} in ${metadataPath}`,
    );
  const fileSha512 = yamlValue(fileEntry[1]);
  if (pathValue !== appImageName)
    throw new Error(
      `Metadata path ${pathValue} does not match ${appImageName}`,
    );
  if (sha512 !== fileSha512)
    throw new Error('Top-level sha512 does not match the AppImage files entry');

  const actualSha512 = crypto
    .createHash('sha512')
    .update(await fs.readFile(path.join(artifactDirectory, appImageName)))
    .digest('base64');
  if (actualSha512 !== sha512)
    throw new Error(`Metadata sha512 does not match ${appImageName}`);
  console.log(
    `Validated Linux artifacts and updater metadata for ${appImageName}`,
  );
}

void main();
