import { shellResultView } from '../shell-commands';

export type ShellActionHandlerDeps = {
  runShellCommand: (command: any, args?: any[], options?: any) => Promise<any>;
  runShellScript: (script: any, options?: any) => Promise<any>;
};

export function createShellActionHandlers(deps: ShellActionHandlerDeps) {
  const { runShellCommand, runShellScript } = deps;

  async function handleShellExec(action) {
    const result = await runShellCommand(
      action.command,
      action.args || [],
      action.options || {},
    );
    return {
      view: shellResultView(
        action.title || action.command || 'Command',
        result,
      ),
      navigation: 'push',
    };
  }

  async function handleShellScript(action) {
    const result = await runShellScript(action.script, action.options || {});
    return {
      view: shellResultView(action.title || 'Script', result),
      navigation: 'push',
    };
  }

  return { handleShellExec, handleShellScript };
}
