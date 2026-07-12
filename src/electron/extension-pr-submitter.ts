import fs from 'node:fs/promises';
import path from 'node:path';

export type ExtensionPrSubmitterDeps = {
  execFileText: (
    command: string,
    args?: string[],
    options?: { encoding?: string },
  ) => Promise<string>;
  extensionsDir: string;
  repoOwner: string;
  repoName: string;
  logInfo: (message: string, data?: unknown) => void;
  logWarn: (message: string, data?: unknown) => void;
};

export type GhStatus = { installed: boolean; authed: boolean };

export type SubmitResult = { ok: boolean; message: string; prUrl?: string };

function sourceExtensionSlug(filePath: string): string {
  return path.basename(filePath, path.extname(filePath));
}

function createExtensionNameFromSlug(slug: string): string {
  return slug
    .split(/[-_]/)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join('');
}

function createFactoryFunctionName(slug: string): string {
  return `create${createExtensionNameFromSlug(slug)}Extension`;
}

function createExtensionModuleTitle(source: string, slug: string): string {
  const match = source.match(/title:\s*["'`]([^"'`]+)/);
  return match ? match[1] : slug;
}

export function createExtensionPrSubmitter(deps: ExtensionPrSubmitterDeps) {
  let cachedGhStatus: GhStatus | null = null;

  async function probeGh(): Promise<GhStatus> {
    if (cachedGhStatus) return cachedGhStatus;
    try {
      await deps.execFileText('gh', ['--version']);
    } catch {
      cachedGhStatus = { installed: false, authed: false };
      return cachedGhStatus;
    }
    try {
      await deps.execFileText('gh', ['auth', 'status']);
      cachedGhStatus = { installed: true, authed: true };
    } catch {
      cachedGhStatus = { installed: true, authed: false };
    }
    return cachedGhStatus;
  }

  async function getMainSha(owner: string, repo: string): Promise<string> {
    const stdout = await deps.execFileText('gh', [
      'api',
      `repos/${owner}/${repo}/branches/main`,
      '--jq',
      '.commit.sha',
    ]);
    return (stdout as string).trim();
  }

  async function fetchIndexTs(
    forkOwner: string,
  ): Promise<{ content: string; sha: string }> {
    const stdout = await deps.execFileText('gh', [
      'api',
      `repos/${forkOwner}/nvm/contents/src/electron/extensions/index.ts`,
    ]);
    const parsed = JSON.parse(stdout as string);
    return {
      content: Buffer.from(parsed.content, 'base64').toString('utf-8'),
      sha: parsed.sha,
    };
  }

  function findClosingBracketAfter(text: string, startPos: number): number {
    let depth = 0;
    for (let i = startPos; i < text.length; i++) {
      if (text[i] === '[') depth++;
      else if (text[i] === ']') {
        depth--;
        if (depth === 0) return i;
      }
    }
    return -1;
  }

  function spliceIntoIndexTs(
    currentIndex: string,
    slug: string,
  ): { updated: string; importLine: string; factoryFunc: string } {
    const factoryFunc = createFactoryFunctionName(slug);
    const importLine = `import { ${factoryFunc} } from './${slug}';`;

    const escapedFactoryFunc = factoryFunc.replace(
      /[.*+?^${}()|[\]\\]/g,
      '\\$&',
    );
    if (new RegExp(`\\b${escapedFactoryFunc}\\b`).test(currentIndex)) {
      return { updated: currentIndex, importLine, factoryFunc };
    }

    const lastImportMatch = currentIndex.match(
      /^import\s+\{[^}]+\}\s+from\s+'[^']+';?$/gm,
    );
    let withImport: string;
    if (lastImportMatch && lastImportMatch.length > 0) {
      const lastLine = lastImportMatch[lastImportMatch.length - 1];
      const insertAt = currentIndex.lastIndexOf(lastLine) + lastLine.length;
      withImport =
        currentIndex.slice(0, insertAt) +
        `\n${importLine}` +
        currentIndex.slice(insertAt);
    } else {
      withImport = `${importLine}\n${currentIndex}`;
    }

    const factoriesStartMatch = withImport.match(
      /\bINTERNAL_EXTENSION_FACTORIES\s*:\s*Array<[^>]*>\s*=\s*\[/,
    );
    if (!factoriesStartMatch)
      return { updated: withImport, importLine, factoryFunc };

    const openBracketIdx =
      factoriesStartMatch.index! + factoriesStartMatch[0].length - 1;
    const closeBracketIdx = findClosingBracketAfter(withImport, openBracketIdx);
    if (closeBracketIdx < 0)
      return { updated: withImport, importLine, factoryFunc };

    const updated =
      withImport.slice(0, closeBracketIdx) +
      `  ${factoryFunc},\n` +
      withImport.slice(closeBracketIdx);
    return { updated, importLine, factoryFunc };
  }

  async function submitExtensionPr(action: any): Promise<SubmitResult> {
    const targetAction = action.targetAction || action;
    if (
      !(
        ['extension-root-item', 'extension-action'].includes(
          targetAction?.kind,
        ) && targetAction?.removable
      )
    ) {
      return {
        ok: false,
        message: 'Only generated extensions can be submitted as PRs',
      };
    }

    const extensionFile = targetAction?.extensionFile;
    if (!extensionFile)
      return { ok: false, message: 'Extension file not found' };

    const resolvedExtensionsDir = path.resolve(deps.extensionsDir);
    const filePath = path.resolve(path.join(deps.extensionsDir, extensionFile));

    if (!filePath.startsWith(resolvedExtensionsDir + path.sep)) {
      return {
        ok: false,
        message: 'Extension must be inside extensions directory',
      };
    }

    let source: string;
    try {
      source = await fs.readFile(filePath, 'utf-8');
    } catch {
      return {
        ok: false,
        message: `Cannot read extension file: ${extensionFile}`,
      };
    }

    const slug = sourceExtensionSlug(filePath);
    const title =
      targetAction.title || createExtensionModuleTitle(source, slug);

    try {
      await deps.execFileText('gh', ['auth', 'status']);
    } catch {
      return {
        ok: false,
        message: 'Sign in to GitHub CLI to submit extensions (gh auth login)',
      };
    }

    const { repoOwner, repoName } = deps;

    try {
      await deps.execFileText('gh', [
        'repo',
        'fork',
        `${repoOwner}/${repoName}`,
        '--remote=false',
      ]);
    } catch (error) {
      deps.logWarn('extension-pr-submitter.fork-failed', { error });
      return {
        ok: false,
        message: 'Failed to fork repository. Check your GitHub CLI setup.',
      };
    }

    let forkOwner: string;
    try {
      const forkOwnerRaw = await deps.execFileText('gh', [
        'api',
        'user',
        '--jq',
        '.login',
      ]);
      forkOwner = (forkOwnerRaw as string).trim();
    } catch (error) {
      deps.logWarn('extension-pr-submitter.fork-owner-failed', { error });
      return {
        ok: false,
        message: 'Failed to resolve fork owner. Check your GitHub CLI setup.',
      };
    }

    let mainSha: string;
    try {
      mainSha = await getMainSha(repoOwner, repoName);
    } catch (error) {
      deps.logWarn('extension-pr-submitter.main-sha-failed', { error });
      return {
        ok: false,
        message: 'Failed to fetch repository metadata.',
      };
    }

    const branchName = `submit-extension-${slug}`;

    try {
      await deps.execFileText('gh', [
        'api',
        '--method',
        'POST',
        `repos/${forkOwner}/nvm/git/refs`,
        '-f',
        `ref=refs/heads/${branchName}`,
        '-f',
        `sha=${mainSha}`,
      ]);
    } catch (error) {
      deps.logWarn('extension-pr-submitter.branch-failed', { error });
      return {
        ok: false,
        message: `Failed to create branch. The branch "${branchName}" may already exist.`,
      };
    }

    try {
      await deps.execFileText('gh', [
        'api',
        '--method',
        'PUT',
        `repos/${forkOwner}/nvm/contents/src/electron/extensions/${slug}.ts`,
        '-f',
        `message=Add extension ${title}`,
        '-f',
        `content=${Buffer.from(source).toString('base64')}`,
        '-f',
        `branch=${branchName}`,
      ]);
    } catch (error) {
      deps.logWarn('extension-pr-submitter.extension-upload-failed', {
        error,
      });
      return {
        ok: false,
        message: 'Failed to upload extension file.',
      };
    }

    let currentIndex: string;
    let indexSha: string;
    try {
      const index = await fetchIndexTs(forkOwner);
      currentIndex = index.content;
      indexSha = index.sha;
    } catch (error) {
      deps.logWarn('extension-pr-submitter.index-fetch-failed', { error });
      return {
        ok: false,
        message: 'Failed to fetch extension barrel index.',
      };
    }

    const { updated: updatedIndex } = spliceIntoIndexTs(currentIndex, slug);

    try {
      await deps.execFileText('gh', [
        'api',
        '--method',
        'PUT',
        `repos/${forkOwner}/nvm/contents/src/electron/extensions/index.ts`,
        '-f',
        'message=Register extension in barrel',
        '-f',
        `content=${Buffer.from(updatedIndex).toString('base64')}`,
        '-f',
        `sha=${indexSha}`,
        '-f',
        `branch=${branchName}`,
      ]);
    } catch (error) {
      deps.logWarn('extension-pr-submitter.index-update-failed', { error });
      return {
        ok: false,
        message: 'Failed to update extension barrel index.',
      };
    }

    let prUrl: string;
    try {
      const prUrlRaw = await deps.execFileText('gh', [
        'pr',
        'create',
        '--repo',
        `${repoOwner}/${repoName}`,
        '--head',
        `${forkOwner}:${branchName}`,
        '--title',
        `Add extension: ${title}`,
        '--body',
        `Submitted from Nevermind. Adds the \`${title}\` extension under \`src/electron/extensions/${slug}.ts\`.`,
      ]);
      prUrl = (prUrlRaw as string).trim();
    } catch (error) {
      deps.logWarn('extension-pr-submitter.pr-create-failed', { error });
      return {
        ok: false,
        message: 'Failed to create pull request.',
      };
    }

    deps.logInfo('extension-pr-submitter.success', { slug, title, prUrl });

    return { ok: true, message: 'PR opened', prUrl };
  }

  return { submitExtensionPr, probe: probeGh };
}
