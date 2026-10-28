import type { ExtensionContext } from '../resources/nevermind-extension-api';
import { showExtensionFeedback } from './feedback';
import {
  WINDOW_MANAGEMENT_SUBTITLE,
  WINDOW_UNAVAILABLE_MESSAGE,
} from './window-management-commands';

type FocusDirection = 'left' | 'right' | 'up' | 'down';

interface FocusPresentation {
  title: string;
  icon: string;
  keywords: string[];
}

const FOCUS_PRESENTATION: Record<FocusDirection, FocusPresentation> = {
  left: {
    title: 'Focus Window Left',
    icon: 'arrow-left',
    keywords: ['focus', 'select', 'left'],
  },
  right: {
    title: 'Focus Window Right',
    icon: 'arrow-right',
    keywords: ['focus', 'select', 'right'],
  },
  up: {
    title: 'Focus Window Up',
    icon: 'arrow-up',
    keywords: ['focus', 'select', 'above', 'up'],
  },
  down: {
    title: 'Focus Window Down',
    icon: 'arrow-down',
    keywords: ['focus', 'select', 'below', 'down'],
  },
};

const FOCUS_DIRECTIONS = Object.keys(FOCUS_PRESENTATION) as FocusDirection[];

function focusLocalId(direction: FocusDirection) {
  return `layout.focus-${direction}`;
}

function reportFocusNeighborResult(
  ctx: Pick<ExtensionContext, 'ui'>,
  title: string,
  result: { ok: boolean; message?: string } | undefined,
) {
  if (!result) {
    showExtensionFeedback(ctx, title, WINDOW_UNAVAILABLE_MESSAGE, 'error');
  } else if (result.ok === false && result.message) {
    showExtensionFeedback(ctx, title, result.message, 'error');
  }
}

async function focusNeighborWindow(
  ctx: ExtensionContext,
  direction: FocusDirection,
) {
  const { title } = FOCUS_PRESENTATION[direction];
  const result = await ctx.desktop.windowManagement?.focusNeighbor(direction);
  reportFocusNeighborResult(ctx, title, result);
}

function focusNeighborAction(direction: FocusDirection) {
  const presentation = FOCUS_PRESENTATION[direction];
  return {
    id: focusLocalId(direction),
    title: presentation.title,
    subtitle: WINDOW_MANAGEMENT_SUBTITLE,
    icon: presentation.icon,
    keywords: presentation.keywords,
    mode: 'noView' as const,
    dismissAfterRun: 'auto' as const,
    run: (innerCtx: ExtensionContext) =>
      focusNeighborWindow(innerCtx, direction),
  };
}

export function windowFocusActions() {
  return FOCUS_DIRECTIONS.map(focusNeighborAction);
}
