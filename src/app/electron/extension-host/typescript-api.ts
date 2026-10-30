import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { API, DiagnosticCategory } from 'typescript/unstable/sync';
import type { Diagnostic, Project } from 'typescript/unstable/sync';
import type { FileSystem } from 'typescript/unstable/fs';
import type { SourceFile } from 'typescript/unstable/ast';

/**
 * Gateway to the TypeScript 7 native checker: one lazily started session
 * shared by extension manifest parsing and extension typechecking. Sources
 * that never touch disk are served through an in-memory overlay; files on
 * disk are checked through a throwaway virtual tsconfig so callers control
 * the compiler options instead of a discovered config.
 */

export * from 'typescript/unstable/ast';

const virtualFiles = new Map<string, string>();
let checkerSession: API | undefined;
let virtualSequence = 0;

/**
 * Normalizes both sides of every overlay lookup so virtual paths survive the
 * checker's own path normalization on Windows.
 */
function virtualKey(filePath: string): string {
  const normalized = path.normalize(filePath);
  return process.platform === 'win32' ? normalized.toLowerCase() : normalized;
}

const overlay: FileSystem = {
  readFile: (fileName) => virtualFiles.get(virtualKey(fileName)),
  fileExists: (fileName) =>
    virtualFiles.has(virtualKey(fileName)) ? true : undefined,
  directoryExists: () => undefined,
  getAccessibleEntries: () => undefined,
};

/**
 * Packaged builds stage the checker under `Resources/typescript-native`
 * (scripts/after-pack.cjs) because binaries cannot spawn from inside the
 * asar archive. Dev and test runs leave it unset so the typescript package
 * resolves its own platform binary from node_modules.
 */
function stagedCheckerPath(): string | undefined {
  const here = fileURLToPath(import.meta.url);
  if (!here.includes(`${path.sep}app.asar${path.sep}`)) return undefined;
  const { resourcesPath } = process;
  return path.join(
    resourcesPath,
    'typescript-native',
    'lib',
    process.platform === 'win32' ? 'tsc.exe' : 'tsc',
  );
}

function getSession(): API {
  if (!checkerSession) {
    checkerSession = new API({
      cwd: process.cwd(),
      fs: overlay,
      tsserverPath: stagedCheckerPath(),
    });
  }
  return checkerSession;
}

/**
 * Parses source text that never exists on disk. Positions in the returned
 * tree index into `source`, so callers can slice the original text with them.
 */
export function parseTypeScriptSource(
  fileName: string,
  source: string,
): SourceFile {
  virtualSequence += 1;
  const virtualPath = path.join(
    path.dirname(fileURLToPath(import.meta.url)),
    `nvm-parse-${virtualSequence}`,
    path.basename(fileName),
  );
  virtualFiles.set(virtualKey(virtualPath), source);
  const client = getSession();
  const snapshot = client.updateSnapshot({ openFiles: [virtualPath] });
  try {
    const sourceFile = snapshot
      .getDefaultProjectForFile(virtualPath)
      ?.program.getSourceFile(virtualPath);
    if (!sourceFile) throw new Error(`Unable to parse ${fileName}`);
    return sourceFile;
  } finally {
    snapshot.dispose();
    client.updateSnapshot({ closeFiles: [virtualPath] });
    virtualFiles.delete(virtualKey(virtualPath));
  }
}

export type TypecheckCompilerOptions = Record<string, string | boolean>;

function flattenDiagnosticText(diagnostic: Diagnostic, depth = 0): string {
  const lines = [`${'  '.repeat(depth)}${diagnostic.text}`];
  for (const chained of diagnostic.messageChain ?? []) {
    lines.push(flattenDiagnosticText(chained, depth + 1));
  }
  return lines.join('\n');
}

function formatDiagnostic(project: Project, diagnostic: Diagnostic): string {
  const { fileName, pos } = diagnostic;
  const text = flattenDiagnosticText(diagnostic);
  if (!fileName || pos < 0) return `error TS${diagnostic.code}: ${text}`;
  const location = project.program
    .getSourceFile(fileName)
    ?.getLineAndCharacterOfPosition(pos);
  if (!location) return `${fileName}: error TS${diagnostic.code}: ${text}`;
  return `${fileName}(${location.line + 1},${location.character + 1}): error TS${diagnostic.code}: ${text}`;
}

/**
 * Typechecks the given on-disk files as one project and returns one plain
 * text line per error. The virtual tsconfig uses absolute roots so callers
 * may mix files from different directories.
 */
export function typecheckTypeScriptFiles(
  filePaths: readonly string[],
  compilerOptions: TypecheckCompilerOptions,
): string[] {
  if (!filePaths.length) return [];
  virtualSequence += 1;
  const configPath = path.join(
    path.dirname(filePaths[0]),
    `tsconfig.nvm-typecheck-${virtualSequence}.json`,
  );
  virtualFiles.set(
    virtualKey(configPath),
    JSON.stringify({ compilerOptions, files: [...filePaths] }, null, 2),
  );
  const client = getSession();
  const snapshot = client.updateSnapshot({
    openProjects: [configPath],
    fileChanges: { invalidateAll: true },
  });
  try {
    const project = snapshot.getProject(configPath);
    if (!project) throw new Error('Unable to open typecheck project');
    const diagnostics: Diagnostic[] = [];
    for (const filePath of filePaths) {
      diagnostics.push(
        ...project.program.getSyntacticDiagnostics(filePath),
        ...project.program.getSemanticDiagnostics(filePath),
        ...project.program.getDeclarationDiagnostics(filePath),
      );
    }
    diagnostics.push(
      ...project.program.getProgramDiagnostics(),
      ...project.program.getGlobalDiagnostics(),
    );
    return diagnostics
      .filter((diagnostic) => diagnostic.category === DiagnosticCategory.Error)
      .map((diagnostic) => formatDiagnostic(project, diagnostic));
  } finally {
    snapshot.dispose();
    client.updateSnapshot({ closeProjects: [configPath] });
    virtualFiles.delete(virtualKey(configPath));
  }
}
