import type {
  ExtensionContext,
  ExtensionWindowLayoutSpec,
  NevermindExtension,
} from '../resources/nevermind-extension-api';
import { showExtensionFeedback } from './feedback';

const WINDOW_MANAGEMENT_OS_CAPABILITY = 'window-management';
const WINDOW_MANAGEMENT_SUBTITLE = 'Window Management';
const WINDOW_UNAVAILABLE_MESSAGE =
  'Window management is not available on this system.';
const WINDOW_LAYOUT_KEYWORDS = ['snap', 'resize', 'window', 'tile'];

interface WindowLayout {
  id: string;
  title: string;
  icon: string;
  keywords: string[];
  spec: ExtensionWindowLayoutSpec;
}

const WINDOW_LAYOUTS: WindowLayout[] = [
  {
    id: 'left-half',
    title: 'Left Half',
    icon: 'panel-left',
    keywords: ['left', 'half'],
    spec: { kind: 'fraction', x: 0, y: 0, width: 0.5, height: 1 },
  },
  {
    id: 'right-half',
    title: 'Right Half',
    icon: 'panel-right',
    keywords: ['right', 'half'],
    spec: { kind: 'fraction', x: 0.5, y: 0, width: 0.5, height: 1 },
  },
  {
    id: 'top-half',
    title: 'Top Half',
    icon: 'panel-top',
    keywords: ['top', 'half'],
    spec: { kind: 'fraction', x: 0, y: 0, width: 1, height: 0.5 },
  },
  {
    id: 'bottom-half',
    title: 'Bottom Half',
    icon: 'panel-bottom',
    keywords: ['bottom', 'half'],
    spec: { kind: 'fraction', x: 0, y: 0.5, width: 1, height: 0.5 },
  },
  {
    id: 'maximize',
    title: 'Maximize',
    icon: 'maximize',
    keywords: ['max', 'full', 'fill'],
    spec: { kind: 'fraction', x: 0, y: 0, width: 1, height: 1 },
  },
  {
    id: 'center',
    title: 'Center',
    icon: 'scan',
    keywords: ['middle', 'centre'],
    spec: { kind: 'center' },
  },
];

function windowLayoutLocalId(layout: WindowLayout) {
  return `layout.${layout.id}`;
}

function reportWindowLayoutFailure(
  ctx: ExtensionContext,
  layout: WindowLayout,
  message: string,
) {
  showExtensionFeedback(ctx, layout.title, message, 'error');
}

async function applyWindowLayout(ctx: ExtensionContext, layout: WindowLayout) {
  const result = await ctx.desktop.windowManagement?.apply(layout.spec);
  if (!result) {
    reportWindowLayoutFailure(ctx, layout, WINDOW_UNAVAILABLE_MESSAGE);
  } else if (!result.ok) {
    reportWindowLayoutFailure(ctx, layout, result.message);
  }
}

function windowLayoutAction(layout: WindowLayout) {
  return {
    id: windowLayoutLocalId(layout),
    title: layout.title,
    subtitle: WINDOW_MANAGEMENT_SUBTITLE,
    icon: layout.icon,
    keywords: [...WINDOW_LAYOUT_KEYWORDS, ...layout.keywords],
    mode: 'noView' as const,
    dismissAfterRun: 'auto' as const,
    run: (ctx: ExtensionContext) => applyWindowLayout(ctx, layout),
  };
}

export function createWindowManagementExtension() {
  return {
    id: 'window-management',
    title: 'Window Management',
    capabilities: ['desktop.window-management'],
    actions(ctx) {
      if (!ctx.system.capabilities.has(WINDOW_MANAGEMENT_OS_CAPABILITY)) {
        return [];
      }
      return WINDOW_LAYOUTS.map(windowLayoutAction);
    },
  } satisfies NevermindExtension;
}
