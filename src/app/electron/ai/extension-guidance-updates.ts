import fs from 'node:fs/promises';

type GuidanceRule = { pattern: RegExp; also?: RegExp; finding: string };

const OUTDATED_GUIDANCE_RULES: GuidanceRule[] = [
  {
    pattern: /(?:\bCmd|\bCommand|⌘)\s*\+?\s*K\b/,
    finding:
      'It writes Cmd+K into visible text. The host already shows the correct action-menu shortcut for each platform, so remove it.',
  },
  {
    pattern: /from the action menu|open the action menu/i,
    finding:
      'It tells users to open the action menu. Lists and progress views now show their first action as a row that Enter runs, so drop that instruction.',
  },
  {
    pattern: /['"`](?:python3?|node)['"`]/,
    also: /write|open\(/,
    finding:
      'It saves files through a Python or Node shell command, which fails when that tool is missing. Use ctx.desktop.files.writeText instead.',
  },
  {
    pattern: /ctx\.ui\.progress\(/,
    also: /value:\s*[^,\n]*\+\s*1\b/,
    finding:
      'Its progress value counts the current step as finished, so the bar can read 100% while a step is still running. Count completed steps or omit value and total.',
  },
  {
    pattern: /previewView\(/,
    also: /\]\s*\.join\(\s*['"`]\\n['"`]\s*\)/,
    finding:
      'It joins preview lines with single newlines, which Markdown shows as one line. Use a list or blank lines between values.',
  },
];

export function outdatedGuidanceFindings(source: string) {
  return OUTDATED_GUIDANCE_RULES.filter(
    (rule) =>
      rule.pattern.test(source) && (!rule.also || rule.also.test(source)),
  ).map((rule) => rule.finding);
}

export function guidanceUpdatePrompt(findings: string[]) {
  if (!findings.length) return '';
  return [
    'Update this extension to current Nevermind guidance without changing what it does:',
    ...findings.map((finding) => `- ${finding}`),
  ].join('\n');
}

/** Suggested (not sent) prompt for a builder chat whose extension predates current guidance. */
export async function guidanceUpdatePromptForFile(filePath: string) {
  const source = await fs.readFile(filePath, 'utf8').catch(() => '');
  return guidanceUpdatePrompt(outdatedGuidanceFindings(source));
}
