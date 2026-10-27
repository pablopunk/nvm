export interface DocsTag {
  name: string;
  text: string;
}

export interface ApiMember {
  path: string;
  doc: string;
  signature: string;
  deprecated: boolean;
}

export interface ApiEntry {
  name: string;
  doc: string;
  tags: DocsTag[];
  deprecated: boolean;
  /** Full declaration, present only when it is short enough to read inline. */
  source?: string;
  /** Flattened members, e.g. `ui.list` for `ctx.ui.list`. */
  members: ApiMember[];
}

export interface FeatureCommand {
  title: string;
  subtitle?: string;
  aliases: string[];
}

export interface FeatureExtension {
  id: string;
  title: string;
  description?: string;
  capabilities: string[];
  commands: FeatureCommand[];
}

export interface DocsData {
  intro: string;
  api: ApiEntry[];
  features: FeatureExtension[];
}

export const emptyDocsData: DocsData = { intro: '', api: [], features: [] };

/**
 * `src/generated/docs-data.json` is produced by `scripts/generate-docs-data.ts`
 * before every dev and build run. A glob keeps pages building if it is absent.
 */
const generatedModules = import.meta.glob<DocsData>(
  '../generated/docs-data.json',
  { eager: true, import: 'default' },
);

export const docsData: DocsData =
  Object.values(generatedModules)[0] ?? emptyDocsData;
