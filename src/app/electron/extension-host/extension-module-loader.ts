import { pathToFileURL } from 'node:url';
import {
  operationStage,
  recordOperationFailure,
  runOperation,
} from '../diagnostics/observability';
import { info as logInfo } from '../platform/logger';

export async function loadExtensionModule(fullPath) {
  return runOperation('extension.load', async function importExtensionModule() {
    operationStage('load');
    const url = pathToFileURL(fullPath);
    url.searchParams.set('reload', String(Date.now()));
    try {
      const imported = await import(url.href);
      return imported.default || imported;
    } catch (error) {
      const record = recordOperationFailure(error, 'load');
      logInfo(
        'diagnostic.module',
        { reference: record?.reference, filePath: fullPath },
        { source: 'host', scope: 'diagnostics' },
      );
      throw error;
    }
  });
}
