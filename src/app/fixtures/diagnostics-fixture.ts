import type { NevermindExtension } from '../resources/nevermind-extension-api';
import { diagnosticRecordsView } from '../extensions/diagnostics';

const extension: NevermindExtension = {
  id: 'dev.diagnostics',
  title: 'Recent Errors',
  commands: [
    {
      id: 'recent-errors',
      title: 'Recent Errors',
      icon: 'circle-alert',
      run: (ctx) =>
        diagnosticRecordsView(ctx, [
          {
            reference: 'a'.repeat(32),
            bootId: 'b'.repeat(32),
            timestamp: new Date().toISOString(),
            build: 'nevermind@0.20.0+fixture',
            operation: 'view.render',
            stage: 'render',
            outcome: 'failed',
            process: 'renderer',
            reporting: 'capture_requested',
          },
          {
            reference: 'c'.repeat(32),
            bootId: 'b'.repeat(32),
            timestamp: new Date().toISOString(),
            build: 'nevermind@0.20.0+fixture',
            operation: 'ai.stream',
            stage: 'stream',
            outcome: 'failed',
            process: 'main',
            reporting: 'local_only',
          },
        ]),
    },
  ],
};

// biome-ignore lint/style/noDefaultExport: Dev fixture modules use the extension loader contract.
export default extension;
