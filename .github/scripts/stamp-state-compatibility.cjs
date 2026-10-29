'use strict';

const fs = require('node:fs');
const { epoch } = require('../../src/app/resources/state-compatibility.json');

function stampMetadata(file) {
  if (!Number.isSafeInteger(epoch) || epoch < 1)
    throw new Error('Invalid state compatibility epoch');
  const text = fs.readFileSync(file, 'utf8');
  if (/^nevermindStateEpoch:/m.test(text))
    throw new Error('Metadata already has a state epoch');
  fs.writeFileSync(file, `${text.trimEnd()}\nnevermindStateEpoch: ${epoch}\n`);
}

if (require.main === module) {
  try {
    if (process.argv.length !== 3) throw new Error('Expected metadata path');
    stampMetadata(process.argv[2]);
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}
module.exports = { stampMetadata };
