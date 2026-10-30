'use strict';

// TypeScript 7 ships no in-process parser; source text is parsed by the
// native checker behind the `typescript/unstable/sync` API. This module is
// the repo-checks entry point to that API: it exposes the ast guard surface
// plus a drop-in `parseTypeScriptSource` for the old `ts.createSourceFile`
// call shape. Parsing uses an in-memory overlay file so callers may hand in
// text that never touches disk.
const path = require('node:path');
const { API } = require('typescript/unstable/sync');
const ast = require('typescript/unstable/ast');

const virtualFiles = new Map();
let api;
let virtualSequence = 0;

function virtualFileSystem() {
  return {
    readFile: (fileName) => virtualFiles.get(fileName),
    fileExists: (fileName) => (virtualFiles.has(fileName) ? true : undefined),
    directoryExists: () => undefined,
    getAccessibleEntries: () => undefined,
  };
}

function getApi() {
  if (!api) api = new API({ cwd: process.cwd(), fs: virtualFileSystem() });
  return api;
}

function parseTypeScriptSource(fileName, source) {
  virtualSequence += 1;
  const virtualPath = `/nvm-parse/${virtualSequence}/${path.basename(fileName)}`;
  virtualFiles.set(virtualPath, source);
  const session = getApi();
  const snapshot = session.updateSnapshot({ openFiles: [virtualPath] });
  try {
    const sourceFile = snapshot
      .getDefaultProjectForFile(virtualPath)
      ?.program.getSourceFile(virtualPath);
    if (!sourceFile) throw new Error(`Unable to parse ${fileName}`);
    return sourceFile;
  } finally {
    snapshot.dispose();
    session.updateSnapshot({ closeFiles: [virtualPath] });
    virtualFiles.delete(virtualPath);
  }
}

module.exports = { ...ast, parseTypeScriptSource };
