const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

function desktopBuildIdentity(root = path.resolve(__dirname, '..')) {
  const { version } = JSON.parse(
    fs.readFileSync(path.join(root, 'package.json'), 'utf8'),
  );
  const commit =
    process.env.GITHUB_SHA ||
    execFileSync('git', ['rev-parse', 'HEAD'], {
      cwd: root,
      encoding: 'utf8',
    }).trim();
  if (
    !/^[a-f0-9]{40}$/.test(commit) ||
    !/^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$/.test(version)
  )
    throw new Error('Invalid desktop build identity');
  return `nevermind@${version}+${commit}.${process.platform}`;
}

module.exports = { desktopBuildIdentity };
