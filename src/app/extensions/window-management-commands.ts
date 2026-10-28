import {
  WINDOW_LAYOUT_IDS,
  type WindowLayoutId,
} from '../electron/os/window-layout-catalog';
import type {
  ExtensionContext,
  ExtensionWindowApplyResult,
  ExtensionWindowLayoutSpec,
} from '../resources/nevermind-extension-api';
import { showExtensionFeedback } from './feedback';
import { WINDOW_LAYOUT_PRESENTATION } from './window-management-layout-presentation';
import { windowManagementOptionsFor } from './window-management-options';
import { extensionWindowManagementSettingsStore } from './window-management-settings';
import { windowManagementSettingsAction } from './window-management-settings-action';
import type { WindowManagementSettings } from './window-management-settings-model';
import { windowLayoutSpecFor } from './window-management-specs';

const WINDOW_UNAVAILABLE_MESSAGE =
  'Window management is not available on this system.';
const WINDOW_LAYOUT_KEYWORDS = ['snap', 'tile'];

function reportWindowLayoutResult(
  ctx: Pick<ExtensionContext, 'ui'>,
  title: string,
  result: ExtensionWindowApplyResult | undefined,
) {
  if (!result) {
    showExtensionFeedback(ctx, title, WINDOW_UNAVAILABLE_MESSAGE, 'error');
  } else if (result.ok === false) {
    showExtensionFeedback(ctx, title, result.message, 'error');
  } else if (result.outcome === 'partial' && result.reason) {
    showExtensionFeedback(ctx, title, result.reason);
  }
}

export const WINDOW_MANAGEMENT_SUBTITLE = 'Window Management';
export const WINDOW_MANAGEMENT_OS_CAPABILITY = 'window-management';

export function windowLayoutLocalId(id: WindowLayoutId) {
  return `layout.${id}`;
}

export function windowLayoutKeywords() {
  return [...WINDOW_LAYOUT_KEYWORDS];
}

export async function applyWindowSpecWithSettings(
  ctx: ExtensionContext,
  title: string,
  buildSpec: (settings: WindowManagementSettings) => ExtensionWindowLayoutSpec,
) {
  const settings = await extensionWindowManagementSettingsStore(ctx).load();
  const result = await ctx.desktop.windowManagement?.apply(
    buildSpec(settings),
    windowManagementOptionsFor(settings),
  );
  reportWindowLayoutResult(ctx, title, result);
}

export function applyWindowLayout(ctx: ExtensionContext, id: WindowLayoutId) {
  return applyWindowSpecWithSettings(
    ctx,
    WINDOW_LAYOUT_PRESENTATION[id].title,
    (settings) => windowLayoutSpecFor(id, settings),
  );
}

export function windowLayoutAction(ctx: ExtensionContext, id: WindowLayoutId) {
  const presentation = WINDOW_LAYOUT_PRESENTATION[id];
  return {
    id: windowLayoutLocalId(id),
    title: presentation.title,
    subtitle: WINDOW_MANAGEMENT_SUBTITLE,
    icon: presentation.icon,
    keywords: [...windowLayoutKeywords(), ...presentation.keywords],
    mode: 'noView' as const,
    dismissAfterRun: 'auto' as const,
    actions: [windowManagementSettingsAction(ctx)],
    run: (innerCtx: ExtensionContext) => applyWindowLayout(innerCtx, id),
  };
}

export function windowLayoutActions(ctx: ExtensionContext) {
  return WINDOW_LAYOUT_IDS.map((id) => windowLayoutAction(ctx, id));
}
