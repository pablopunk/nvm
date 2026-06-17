#!/usr/bin/env node
const fs = require('node:fs');
const fsp = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');

const LEARNINGS_FILENAME = 'ai-learnings.md';
const TRACES_FILENAME = 'ai-learning-traces.json';

function timestampLabel() {
  return new Date().toISOString().replace(/[:.]/g, '-').replace(/Z$/, 'Z');
}

function userDataCandidates() {
  if (process.env.NEVERMIND_LEARNING_STORE)
    return [
      {
        learningsPath: process.env.NEVERMIND_LEARNING_STORE,
        tracesPath: process.env.NEVERMIND_LEARNING_STORE.replace(
          /ai-learnings\.md$/,
          TRACES_FILENAME,
        ),
      },
    ];
  if (process.platform === 'darwin') {
    const base = path.join(os.homedir(), 'Library', 'Application Support');
    return ['Nevermind', 'nvm'].map((name) => ({
      learningsPath: path.join(base, name, LEARNINGS_FILENAME),
      tracesPath: path.join(base, name, TRACES_FILENAME),
    }));
  }
  if (process.platform === 'win32') {
    const base =
      process.env.APPDATA || path.join(os.homedir(), 'AppData', 'Roaming');
    return ['Nevermind', 'nvm'].map((name) => ({
      learningsPath: path.join(base, name, LEARNINGS_FILENAME),
      tracesPath: path.join(base, name, TRACES_FILENAME),
    }));
  }
  const base =
    process.env.XDG_CONFIG_HOME || path.join(os.homedir(), '.config');
  return ['Nevermind', 'nvm'].map((name) => ({
    learningsPath: path.join(base, name, LEARNINGS_FILENAME),
    tracesPath: path.join(base, name, TRACES_FILENAME),
  }));
}

function existingStorePath() {
  return (
    userDataCandidates().find(
      (candidate) =>
        fs.existsSync(candidate.learningsPath) ||
        fs.existsSync(candidate.tracesPath),
    ) || userDataCandidates()[0]
  );
}

function summaryMarkdown(state, storePaths) {
  const traces = Array.isArray(state.traces) ? state.traces : [];
  const learnings = Array.isArray(state.learnings) ? state.learnings : [];
  const active = learnings.filter((learning) => learning.status === 'active');
  const recentTraces = [...traces]
    .sort((left, right) => (right.updatedAt || 0) - (left.updatedAt || 0))
    .slice(0, 10);
  const lines = [
    '# Learnings Export',
    '',
    `- Learnings file: \`${storePaths.learningsPath}\``,
    `- Traces file: \`${storePaths.tracesPath}\``,
    `- Exported at: \`${new Date().toISOString()}\``,
    `- Traces: **${traces.length}**`,
    `- Learnings: **${learnings.length}**`,
    `- Active learnings: **${active.length}**`,
    '',
    '## Active Learnings',
    '',
  ];
  if (!active.length) lines.push('_None_');
  else
    active.forEach((learning) =>
      lines.push(`- [${learning.kind}] ${learning.summary}`),
    );
  lines.push('', '## Recent Traces', '');
  if (!recentTraces.length) lines.push('_None_');
  else
    recentTraces.forEach((trace) =>
      lines.push(
        `- ${trace.title || trace.query || trace.chatId}: ${trace.toolCalls?.length || 0} tools, ${trace.extensionEvents?.length || 0} extension events`,
      ),
    );
  lines.push('');
  return `${lines.join('\n')}\n`;
}

async function main() {
  const storePaths = existingStorePath();
  const [learningsText, tracesText] = await Promise.all([
    fsp.readFile(storePaths.learningsPath, 'utf8').catch(() => ''),
    fsp.readFile(storePaths.tracesPath, 'utf8').catch(() => ''),
  ]);
  if (!learningsText && !tracesText) {
    console.error(`No learning store found at ${storePaths.learningsPath}`);
    process.exit(1);
  }
  const learnings = parseLearningsForExport(learningsText);
  const traces = tracesText ? JSON.parse(tracesText).traces || [] : [];
  const state = { learnings, traces };
  const exportDir = path.join(
    process.cwd(),
    '.tmp',
    'learnings-export',
    timestampLabel(),
  );
  await fsp.mkdir(exportDir, { recursive: true });
  await Promise.all([
    fsp.writeFile(
      path.join(exportDir, 'state.json'),
      JSON.stringify(state, null, 2),
    ),
    fsp.writeFile(
      path.join(exportDir, 'traces.json'),
      JSON.stringify(traces, null, 2),
    ),
    fsp.writeFile(
      path.join(exportDir, 'learnings.json'),
      JSON.stringify(learnings, null, 2),
    ),
    learningsText
      ? fsp.writeFile(path.join(exportDir, 'learnings.md'), learningsText)
      : Promise.resolve(),
    fsp.writeFile(
      path.join(exportDir, 'summary.md'),
      summaryMarkdown(state, storePaths),
    ),
  ]);
  console.log(`Exported learnings to ${exportDir}`);
}

function parseLearningsForExport(text) {
  if (!text) return [];
  const learnings = [];
  const blocks = text.split(/^## /m).slice(1);
  for (const block of blocks) {
    const lines = block.split('\n');
    const heading = (lines.shift() || '').toLowerCase();
    const kind = heading.startsWith('workflow')
      ? 'workflow'
      : heading.startsWith('preference')
        ? 'preference'
        : 'environment';
    let current = null;
    for (const rawLine of lines) {
      const line = rawLine.trim();
      if (!line || line === '_None_') continue;
      if (line.startsWith('- ')) {
        if (current?.summary) learnings.push(current);
        current = { kind, summary: line.slice(2).trim(), status: 'active' };
        continue;
      }
      if (!current) continue;
      if (line.startsWith('When: '))
        current.appliesWhen = line.slice('When: '.length).trim();
      else if (line.startsWith('Keywords: '))
        current.keywords = line
          .slice('Keywords: '.length)
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean);
      else if (line.startsWith('Confidence: '))
        current.confidence = line.slice('Confidence: '.length).trim();
      else if (line.startsWith('Evidence: '))
        current.evidence = line.slice('Evidence: '.length).trim();
    }
    if (current?.summary) learnings.push(current);
  }
  return learnings;
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exit(1);
});
