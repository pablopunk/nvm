import type { ExtensionContext } from '../resources/nevermind-extension-api';

export async function refreshWindowManagementContributions(
  ctx: Pick<ExtensionContext, 'actions'>,
) {
  await ctx.actions.refreshContributions().catch(() => undefined);
}
