import { operationTarget } from '../diagnostics/observability';
import { info as logInfo } from '../platform/logger';
import { isInternalExtension } from './internal-extension';

export function createDiagnosticExtensionTargets() {
  const loggedDiagnosticTargets = new Set<string>();

  function setDiagnosticExtensionTarget(extension) {
    const components = {
      'nevermind.apps': 'apps',
      'nevermind.files': 'files',
      'nevermind.clipboard': 'clipboard',
      'nevermind.account': 'account',
      'nevermind.settings': 'settings',
      'nevermind.extensions': 'extensions',
      'nevermind.updates': 'updates',
      'nevermind.diagnostics': 'diagnostics',
      'nevermind.ai-builder': 'ai',
      'nevermind.ai-commands': 'ai',
    } as const;
    const targetId = operationTarget(
      extension,
      isInternalExtension(extension)
        ? (components[extension.id] ?? 'system')
        : 'user-extension',
    );
    if (targetId && !loggedDiagnosticTargets.has(targetId)) {
      loggedDiagnosticTargets.add(targetId);
      if (loggedDiagnosticTargets.size > 100)
        loggedDiagnosticTargets.delete(
          loggedDiagnosticTargets.values().next().value,
        );
      logInfo(
        'diagnostic.target',
        { targetId, extensionId: extension.id, filePath: extension.__filePath },
        { source: 'host', scope: 'diagnostics' },
      );
    }
  }

  return { setDiagnosticExtensionTarget };
}
