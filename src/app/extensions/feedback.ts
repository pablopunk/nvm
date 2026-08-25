import type { ExtensionContext } from '../resources/nevermind-extension-api';

export function showExtensionFeedback(
  ctx: Pick<ExtensionContext, 'ui'>,
  title: string,
  message: string,
  status?: 'success' | 'error',
) {
  ctx.ui.indicator.show({
    id: 'feedback',
    title,
    subtitle: message,
    ...(status ? { status } : {}),
    durationMs: status === 'error' ? 4000 : 2200,
  });
}
