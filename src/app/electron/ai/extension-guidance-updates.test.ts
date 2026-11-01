import assert from 'node:assert/strict';
import test from 'node:test';
import {
  guidanceUpdatePrompt,
  outdatedGuidanceFindings,
} from './extension-guidance-updates';

const OLD_WIZARD = `
  ctx.ui.progress({ label: 'Step', value: currentIndex + 1, total: STEPS.length })
  subtitle: 'Use Cmd+K on a swatch to preview details.'
`;

test('flags the outdated patterns found while dogfooding generated extensions', () => {
  const findings = outdatedGuidanceFindings(OLD_WIZARD);

  assert.equal(findings.length, 2);
  assert.match(findings[0], /Cmd\+K/);
  assert.match(findings[1], /100%/);
});

test('flags shell file writes and single-newline preview content', () => {
  const findings = outdatedGuidanceFindings(`
    const python = 'python3'
    await shell.exec(python, ['-c', "open(p, 'w').write(t)"])
    ctx.ui.previewView({ content: [a, b].join('\\n') })
  `);

  assert.equal(findings.length, 2);
  assert.match(findings[0], /writeText/);
  assert.match(findings[1], /one line/);
});

test('current-guidance extensions get no update prompt', () => {
  const findings = outdatedGuidanceFindings(`
    ctx.ui.progress({ steps, value: completed, total: steps.length })
    ctx.ui.previewView({ content: lines.map((line) => \`- \${line}\`).join('\\n') })
    await ctx.desktop.files.writeText(path, markdown)
  `);

  assert.deepEqual(findings, []);
  assert.equal(guidanceUpdatePrompt(findings), '');
});

test('flags hand-built disabled error rows', () => {
  const findings = outdatedGuidanceFindings(`
    items: [{ id: 'offline', title: 'Could not connect', disabled: true }]
  `);

  assert.deepEqual(findings.length, 1);
  assert.match(findings[0], /ctx\.ui\.error/);
});
