'use strict';

const fs = require('node:fs');

// electron-builder loads afterPack and afterSign into one process, so this
// module's state links the end of afterPack to the start of afterSign. That
// gap is codesigning plus notarization, which no builder log reports.
let afterPackFinishedAt;

function recordPackagingPhase(label, startedAt) {
  const seconds = ((Date.now() - startedAt) / 1000).toFixed(1);
  console.log(`Packaging timing: ${label} took ${seconds}s`);
  const summary = process.env.GITHUB_STEP_SUMMARY;
  if (summary) fs.appendFileSync(summary, `- ${label}: ${seconds}s\n`);
}

function markAfterPackFinished() {
  afterPackFinishedAt = Date.now();
}

function recordSigningSinceAfterPack(label) {
  if (afterPackFinishedAt === undefined) return;
  recordPackagingPhase(label, afterPackFinishedAt);
  afterPackFinishedAt = undefined;
}

module.exports = {
  markAfterPackFinished,
  recordPackagingPhase,
  recordSigningSinceAfterPack,
};
