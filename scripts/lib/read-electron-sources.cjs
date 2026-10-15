'use strict';

const fs = require('node:fs');
const path = require('node:path');

const ELECTRON_SOURCE_SEPARATOR = '\u0000';

function isElectronSourceFile(fileName) {
  return /\.tsx?$/.test(fileName) && !/\.test\.tsx?$/.test(fileName);
}

function walkFiles(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const entryPath = path.join(directory, entry.name);
    return entry.isDirectory() ? walkFiles(entryPath) : [entryPath];
  });
}

function compareMainFirstThenPath(electronDir) {
  const mainPath = path.join(electronDir, 'main.ts');
  return (left, right) => {
    if (left === mainPath) return -1;
    if (right === mainPath) return 1;
    return left < right ? -1 : left > right ? 1 : 0;
  };
}

function listElectronSourceFiles(root = process.cwd()) {
  const electronDir = path.join(root, 'src', 'app', 'electron');
  return walkFiles(electronDir)
    .filter((file) => isElectronSourceFile(path.basename(file)))
    .sort(compareMainFirstThenPath(electronDir));
}

function readElectronSources(root = process.cwd()) {
  return listElectronSourceFiles(root)
    .map((file) => fs.readFileSync(file, 'utf8'))
    .join(ELECTRON_SOURCE_SEPARATOR);
}

module.exports = {
  ELECTRON_SOURCE_SEPARATOR,
  listElectronSourceFiles,
  readElectronSources,
};
