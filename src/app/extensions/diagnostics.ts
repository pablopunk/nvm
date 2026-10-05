import type { ExtensionContext } from '../resources/nevermind-extension-api';
import type { DiagnosticRecord } from '../shared/diagnostics';
import { extensionContext } from './_context';

function reportingLabel(record: DiagnosticRecord) {
  if (record.reporting === 'local_only') return 'Stored on this computer';
  if (record.reporting === 'dropped') return 'Not reported';
  return 'Reporting requested · remote availability unknown';
}

function diagnosticItem(ctx: ExtensionContext, record: DiagnosticRecord) {
  const copyReference = ctx.actions.copyText(
    record.reference,
    'Copy Error Reference',
  );
  const copyDiagnostics = ctx.actions.copyText(
    JSON.stringify(record, null, 2),
    'Copy Safe Diagnostics',
  );
  return ctx.ui.item({
    id: record.reference,
    title:
      record.operation === 'support.report'
        ? 'Problem report'
        : record.operation,
    subtitle: `${new Date(record.timestamp).toLocaleString()} · ${record.stage} · ${record.build}`,
    icon:
      record.operation === 'support.report' ? 'message-circle' : 'circle-alert',
    accessories: [{ text: reportingLabel(record) }],
    primaryAction: copyReference,
    actions: [copyReference, copyDiagnostics],
  });
}

export function diagnosticRecordsView(
  ctx: ExtensionContext,
  records: DiagnosticRecord[],
) {
  return ctx.ui.list({
    id: 'recent-errors',
    title: 'Recent Errors',
    presentation: 'root',
    searchBarPlaceholder: 'Search recent errors',
    subtitle: 'Technical details only · remote availability is not confirmed',
    emptyView: { title: 'No recent errors recorded' },
    items: records.map((record) => diagnosticItem(ctx, record)),
  });
}

function recentErrorsView(ctx: ExtensionContext) {
  return diagnosticRecordsView(ctx, extensionContext.diagnostics.recent());
}

function reportProblem(ctx: ExtensionContext) {
  extensionContext.diagnostics.report();
  return recentErrorsView(ctx);
}

export function createDiagnosticsExtension() {
  return {
    id: 'nevermind.diagnostics',
    title: 'Diagnostics',
    capabilities: [] as const,
    commands: [
      {
        id: 'recent-errors',
        title: 'Show Recent Errors',
        subtitle: 'Find and copy safe error references',
        icon: 'circle-alert',
        run: recentErrorsView,
      },
      {
        id: 'report-problem',
        title: 'Report a Problem',
        subtitle: 'Record a technical problem reference without screen content',
        icon: 'message-circle',
        run: reportProblem,
      },
    ],
  };
}
