import path from 'node:path';
import {
  recordExtensionReadCoverage,
  type BuilderReadPage,
  type ExtensionReadCoverage,
} from './ai-builder-read-page';
import { createBuilderReadTools } from './ai-builder-read-tools';
import { createBuilderWriteTools } from './ai-builder-write-tools';
import type { BuilderToolOptions } from './ai-builder-tool-shared';

type PiApi = typeof import('@earendil-works/pi-coding-agent');
type TypeApi = typeof import('@earendil-works/pi-ai').Type;

export function createBuilderTools(
  pi: PiApi,
  Type: TypeApi,
  options: Omit<BuilderToolOptions, 'recordRead'>,
) {
  const readFiles = new Set<string>();
  const readCoverage = new Map<string, ExtensionReadCoverage>();
  function recordRead(filename: string, page: BuilderReadPage) {
    const basename = path.basename(filename);
    if (readCoverage.get(basename)?.revision !== page.revision)
      readFiles.delete(basename);
    if (recordExtensionReadCoverage(readCoverage, basename, page))
      readFiles.add(basename);
  }
  const toolOptions: BuilderToolOptions = { ...options, recordRead };
  return [
    ...createBuilderReadTools(pi, Type, toolOptions),
    ...createBuilderWriteTools(pi, Type, toolOptions, readFiles),
  ];
}
