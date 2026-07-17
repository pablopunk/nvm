import fs from 'node:fs/promises';
import path from 'node:path';
import { inspectExtensionManifest } from './extension-manifest';

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

type ParsedSource = {
  id: string;
  title: string;
  idStart: number;
  idEnd: number;
};

function parseExtensionSource(source: string): ParsedSource | null {
  const manifest = inspectExtensionManifest(source);
  if (!manifest.id || !manifest.title || !manifest.idStart || !manifest.idEnd)
    return null;
  return {
    id: manifest.id,
    title: manifest.title,
    idStart: manifest.idStart,
    idEnd: manifest.idEnd,
  };
}

export function extensionSlug(title: string): string | null {
  const ascii = title
    .normalize('NFKD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase();
  let slug = ascii.replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
  if (!slug) return null;
  if (/^\d/.test(slug)) slug = `extension-${slug}`;
  return /^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$/.test(slug) ? slug : null;
}

export function factoryName(slug: string): string {
  return `create${slug
    .split('-')
    .map((part) => part[0].toUpperCase() + part.slice(1))
    .join('')}Extension`;
}

function readJsonContent(raw: string): { content: string; sha: string } {
  const parsed = JSON.parse(raw);
  return {
    content: Buffer.from(parsed.content, 'base64').toString('utf8'),
    sha: parsed.sha,
  };
}

export function createExtensionPrSubmitter(deps: ExtensionPrSubmitterDeps) {
  let cachedGhStatus: GhStatus | null = null;
  const gh = (args: string[]) => deps.execFileText('gh', args);
  async function probeGh(): Promise<GhStatus> {
    if (cachedGhStatus) return cachedGhStatus;
    try {
      await gh(['--version']);
    } catch {
      return (cachedGhStatus = { installed: false, authed: false });
    }
    try {
      await gh(['auth', 'status']);
      return (cachedGhStatus = { installed: true, authed: true });
    } catch {
      return (cachedGhStatus = { installed: true, authed: false });
    }
  }
  async function getContent(
    owner: string,
    repo: string,
    file: string,
    ref: string,
  ) {
    return readJsonContent(
      await gh([
        'api',
        `repos/${owner}/${repo}/contents/${file}?ref=${encodeURIComponent(ref)}`,
      ]),
    );
  }
  function updateIndex(index: string, slug: string): string {
    const fn = factoryName(slug);
    const importLine = `import { ${fn} } from './${slug}';`;
    const imports = [...index.matchAll(/^import .*;\s*$/gm)];
    const lastImport = imports.at(-1);
    const withImport = lastImport
      ? `${index.slice(0, (lastImport.index ?? 0) + lastImport[0].length)}\n${importLine}${index.slice((lastImport.index ?? 0) + lastImport[0].length)}`
      : `${importLine}\n${index}`;
    const marker = /INTERNAL_EXTENSION_FACTORIES\s*:\s*Array<[^>]+>\s*=\s*\[/;
    const match = marker.exec(withImport);
    if (!match) return withImport;
    const at = match.index + match[0].length;
    return `${withImport.slice(0, at)}\n  ${fn},${withImport.slice(at)}`;
  }
  function hasIndexCollision(index: string, slug: string): boolean {
    const fn = factoryName(slug);
    return new RegExp(
      `(?:from ['\"]\\./${slug}['\"]|\\b${fn}\\b|['\"]${slug}['\"])`,
    ).test(index);
  }
  async function submitExtensionPr(action: any): Promise<SubmitResult> {
    const targetAction = action.targetAction || action;
    if (
      !['extension-root-item', 'extension-action'].includes(
        targetAction?.kind,
      ) ||
      !targetAction?.removable
    )
      return {
        ok: false,
        message: 'Only generated extensions can be submitted as PRs',
      };
    const extensionFile = targetAction?.extensionFile;
    if (!extensionFile)
      return { ok: false, message: 'Extension file not found' };
    const baseDir = path.resolve(deps.extensionsDir);
    const filePath = path.resolve(path.join(deps.extensionsDir, extensionFile));
    if (!filePath.startsWith(`${baseDir}${path.sep}`))
      return {
        ok: false,
        message: 'Extension must be inside extensions directory',
      };
    let source: string;
    try {
      source = await fs.readFile(filePath, 'utf8');
    } catch {
      return {
        ok: false,
        message: `Cannot read extension file: ${extensionFile}`,
      };
    }
    const parsed = parseExtensionSource(source);
    if (!parsed)
      return {
        ok: false,
        message:
          'Extension must export one object with static top-level id and title strings',
      };
    const slug = extensionSlug(parsed.title);
    if (!slug)
      return {
        ok: false,
        message: 'Extension title must contain ASCII letters or numbers',
      };
    const fn = factoryName(slug);
    const promotedSource = `${source.slice(0, parsed.idStart)}'${slug}'${source.slice(parsed.idEnd)}`;
    const { repoOwner, repoName } = deps;
    try {
      await gh(['auth', 'status']);
    } catch {
      return {
        ok: false,
        message: 'Sign in to GitHub CLI to submit extensions (gh auth login)',
      };
    }
    let targetOwner: string;
    try {
      targetOwner = (await gh(['api', 'user', '--jq', '.login'])).trim();
    } catch (error) {
      deps.logWarn('extension-pr-submitter.current-user-failed', { error });
      return {
        ok: false,
        message:
          'Failed to resolve GitHub account. Check your GitHub CLI setup.',
      };
    }

    let mainSha: string;
    let tree: any;
    let index: { content: string; sha: string };
    try {
      mainSha = (
        await gh([
          'api',
          `repos/${repoOwner}/${repoName}/branches/main`,
          '--jq',
          '.commit.sha',
        ])
      ).trim();
      tree =
        JSON.parse(
          await gh([
            'api',
            `repos/${repoOwner}/${repoName}/git/trees/${mainSha}?recursive=1`,
          ]),
        ).tree || [];
      index = await getContent(
        repoOwner,
        repoName,
        'src/electron/extensions/index.ts',
        mainSha,
      );
    } catch (error) {
      deps.logWarn('extension-pr-submitter.preflight-read-failed', { error });
      return {
        ok: false,
        message: 'Failed to read the upstream repository at main.',
      };
    }
    const outputPath = `src/electron/extensions/${slug}.ts`;
    if (
      tree.some((entry: any) => entry.path === outputPath) ||
      hasIndexCollision(index.content, slug)
    )
      return {
        ok: false,
        message: `Extension name collision: ${slug} already exists`,
      };
    const sourceEntries = tree.filter(
      (entry: any) =>
        entry.type === 'blob' &&
        /^src\/electron\/extensions\/[^/]+\.ts$/.test(entry.path),
    );
    for (const entry of sourceEntries) {
      try {
        const blob = JSON.parse(
          await gh([
            'api',
            `repos/${repoOwner}/${repoName}/git/blobs/${entry.sha}`,
          ]),
        );
        const existing = parseExtensionSource(
          Buffer.from(blob.content, blob.encoding || 'base64').toString('utf8'),
        );
        if (existing?.id === slug)
          return {
            ok: false,
            message: `Extension ID collision: ${slug} already exists`,
          };
      } catch (error) {
        deps.logWarn('extension-pr-submitter.source-scan-failed', {
          error,
          path: entry.path,
        });
        return {
          ok: false,
          message: 'Failed to scan existing built-in extensions.',
        };
      }
    }
    const branchName = `submit-extension-${slug}`;
    try {
      const existingRef = await gh([
        'api',
        `repos/${targetOwner}/${repoName}/git/refs/heads/${branchName}`,
      ]);
      if (existingRef.trim())
        return {
          ok: false,
          message: `Submission branch "${branchName}" already exists; choose a different title`,
        };
    } catch {
      /* expected 404 */
    }
    if (targetOwner !== repoOwner) {
      try {
        await gh(['repo', 'fork', `${repoOwner}/${repoName}`]);
      } catch (error) {
        deps.logWarn('extension-pr-submitter.fork-failed', { error });
        return {
          ok: false,
          message: 'Failed to fork repository. Check your GitHub CLI setup.',
        };
      }
    }
    try {
      await gh([
        'api',
        '--method',
        'POST',
        `repos/${targetOwner}/${repoName}/git/refs`,
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
      await gh([
        'api',
        '--method',
        'PUT',
        `repos/${targetOwner}/${repoName}/contents/${outputPath}`,
        '-f',
        `message=Add extension ${parsed.title}`,
        '-f',
        `content=${Buffer.from(promotedSource).toString('base64')}`,
        '-f',
        `branch=${branchName}`,
      ]);
    } catch (error) {
      deps.logWarn('extension-pr-submitter.extension-upload-failed', { error });
      return { ok: false, message: 'Failed to upload extension file.' };
    }
    try {
      await gh([
        'api',
        '--method',
        'PUT',
        `repos/${targetOwner}/${repoName}/contents/src/electron/extensions/index.ts`,
        '-f',
        'message=Register extension in barrel',
        '-f',
        `content=${Buffer.from(updateIndex(index.content, slug)).toString('base64')}`,
        '-f',
        `sha=${index.sha}`,
        '-f',
        `branch=${branchName}`,
      ]);
    } catch (error) {
      deps.logWarn('extension-pr-submitter.index-update-failed', { error });
      return { ok: false, message: 'Failed to update extension barrel index.' };
    }
    let prUrl: string;
    try {
      prUrl = (
        await gh([
          'pr',
          'create',
          '--repo',
          `${repoOwner}/${repoName}`,
          '--head',
          targetOwner === repoOwner
            ? branchName
            : `${targetOwner}:${branchName}`,
          '--title',
          `Add extension: ${parsed.title}`,
          '--body',
          `Submitted from Nevermind. Adds the \`${parsed.title}\` extension under \`${outputPath}\`.`,
        ])
      ).trim();
    } catch (error) {
      deps.logWarn('extension-pr-submitter.pr-create-failed', { error });
      return { ok: false, message: 'Failed to create pull request.' };
    }
    deps.logInfo('extension-pr-submitter.success', {
      slug,
      title: parsed.title,
      prUrl,
      factory: fn,
    });
    return { ok: true, message: 'PR opened', prUrl };
  }
  return { submitExtensionPr, probe: probeGh };
}
