import { createRequire } from 'node:module';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import type { Model as PiModel } from '@earendil-works/pi-ai';

type PiApi = typeof import('@earendil-works/pi-coding-agent');

export type PiRuntimeModuleResolution = {
  piCodingAgent: string;
  piAi: string;
  piWebAccess: string;
};

export const PI_AI_COMPACTION_RESERVE_TOKENS = 16_384;

export function compactionSettingsForModel(pi: PiApi, model: PiModel) {
  return {
    enabled: true,
    reserveTokens: PI_AI_COMPACTION_RESERVE_TOKENS,
    keepRecentTokens: Math.min(
      pi.DEFAULT_COMPACTION_SETTINGS.keepRecentTokens,
      Math.max(
        1,
        Math.floor(model.contextWindow - PI_AI_COMPACTION_RESERVE_TOKENS - 1),
      ),
    ),
  };
}

export async function findPiWebAccessPath() {
  const require = createRequire(import.meta.url);
  try {
    return path.dirname(require.resolve('pi-web-access/package.json'));
  } catch (error) {
    throw new Error('Bundled Pi web access is unavailable', { cause: error });
  }
}

export function piRuntimeModuleResolution(): PiRuntimeModuleResolution {
  const require = createRequire(import.meta.url);
  return {
    piCodingAgent: import.meta.resolve('@earendil-works/pi-coding-agent'),
    piAi: import.meta.resolve('@earendil-works/pi-ai/compat'),
    piWebAccess: pathToFileURL(require.resolve('pi-web-access/package.json'))
      .href,
  };
}
