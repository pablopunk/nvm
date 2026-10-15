import { performanceTraces } from '../diagnostics/debug-performance';
import {
  bindOperation,
  operationPending,
  runOperation,
} from '../diagnostics/observability';
import { error as logError } from '../platform/logger';

export function runInBackground(task, traceId?: string) {
  operationPending();
  const queuedAt = performance.now();
  setImmediate(
    bindOperation(() => {
      Promise.resolve()
        .then(() =>
          runOperation(
            'os.dispatch',
            () =>
              performanceTraces.run(
                'os.dispatch',
                { queueMs: performance.now() - queuedAt },
                task,
                traceId ? { traceId } : undefined,
              ),
            { defaultOutcome: 'unknown' },
          ),
        )
        .catch((error) => {
          logError('backgroundAction.failed', error, { source: 'host' });
        });
    }),
  );
}
