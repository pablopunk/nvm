import type { DefaultResourceLoader } from '@earendil-works/pi-coding-agent';
import * as logger from '../platform/logger';

export function ensurePiExtensionsLoaded(
  loader: Pick<DefaultResourceLoader, 'getExtensions'>,
  mode: string,
) {
  const loadErrors = loader.getExtensions().errors;
  if (loadErrors.length > 0) {
    logger.error(
      'ai.web-access.load.failed',
      { errors: loadErrors },
      { source: 'host', scope: 'ai' },
    );
    throw new Error(
      `Pi extensions failed to load for ${mode}: ${loadErrors[0].error}`,
    );
  }
}
