'use strict';

const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { verifyUpdaterMetadata } = require('./nightly-assets.cjs');

function assembleMacArtifacts(input, output, version) {
  fs.mkdirSync(output, { recursive: true });
  const files = [];
  let releaseDate;
  for (const arch of ['arm64', 'x64']) {
    const dir = path.join(input, `mac-build-${arch}`);
    const zip = `Nevermind-${version}-mac-${arch}.zip`;
    const names = fs.readdirSync(dir).sort();
    const expected = [zip, `${zip}.blockmap`, 'nightly-mac.yml'].sort();
    if (JSON.stringify(names) !== JSON.stringify(expected)) {
      throw new Error(`Unexpected ${arch} artifact set: ${names.join(', ')}`);
    }
    const text = fs.readFileSync(path.join(dir, 'nightly-mac.yml'), 'utf8');
    const metadata = verifyUpdaterMetadata({
      name: `${arch} nightly-mac.yml`,
      text,
      version,
      assetNames: names,
      sha512Of: (name) =>
        crypto
          .createHash('sha512')
          .update(fs.readFileSync(path.join(dir, name)))
          .digest('base64'),
      sizeOf: (name) => fs.statSync(path.join(dir, name)).size,
    });
    if (metadata.files.length !== 1 || metadata.files[0].url !== zip) {
      throw new Error(`Unexpected ${arch} updater target`);
    }
    const date = /^releaseDate:\s*(.+)$/m.exec(text);
    if (!date) {
      throw new Error(`Missing ${arch} release date`);
    }
    releaseDate ||= date[1];
    files.push(metadata.files[0]);
    for (const name of [zip, `${zip}.blockmap`]) {
      fs.copyFileSync(
        path.join(dir, name),
        path.join(output, name),
        fs.constants.COPYFILE_EXCL,
      );
    }
  }
  const entries = files
    .map(
      (file) =>
        `  - url: ${file.url}\n    sha512: ${file.sha512}\n    size: ${file.size}`,
    )
    .join('\n');
  fs.writeFileSync(
    path.join(output, 'nightly-mac.yml'),
    `version: ${version}\nfiles:\n${entries}\npath: ${files[0].url}\nsha512: ${files[0].sha512}\nreleaseDate: ${releaseDate}\n`,
    { flag: 'wx' },
  );
}

if (require.main === module) {
  const [input, output, version] = process.argv.slice(2);
  if (!(input && output && version)) {
    throw new Error('Expected input, output, and version');
  }
  assembleMacArtifacts(input, output, version);
}
module.exports = { assembleMacArtifacts };
