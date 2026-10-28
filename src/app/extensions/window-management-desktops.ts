import type { ExtensionContext } from '../resources/nevermind-extension-api';
import { showExtensionFeedback } from './feedback';

const LEFT_ARROW_KEY_CODE = 123;

const RIGHT_ARROW_KEY_CODE = 124;

const DESKTOP_SUBTITLE = 'Switch desktop';

const DESKTOP_PRESENTATION: Record<
  DesktopDirection,
  { id: string; title: string; icon: string; keywords: string[] }
> = {
  next: {
    id: 'next-desktop',
    title: 'Next Desktop',
    icon: 'arrow-right',
    keywords: ['space', 'workspace', 'mission control', 'right'],
  },
  previous: {
    id: 'previous-desktop',
    title: 'Previous Desktop',
    icon: 'arrow-left',
    keywords: ['space', 'workspace', 'mission control', 'left'],
  },
};

async function switchDesktop(
  ctx: ExtensionContext,
  direction: DesktopDirection,
) {
  const result = await ctx.desktop.shell?.appleScript(
    desktopSwitchScript(direction),
  );
  if (result?.exitCode === 0) {
    return;
  }
  ctx.logs.error('Desktop switch failed', {
    direction,
    stderr: result?.stderr,
  });
  showExtensionFeedback(
    ctx,
    DESKTOP_PRESENTATION[direction].title,
    DESKTOP_SWITCH_FAILURE_MESSAGE,
    'error',
  );
}

function desktopAction(direction: DesktopDirection) {
  const presentation = DESKTOP_PRESENTATION[direction];
  return {
    id: presentation.id,
    title: presentation.title,
    subtitle: DESKTOP_SUBTITLE,
    icon: presentation.icon,
    keywords: presentation.keywords,
    mode: 'noView' as const,
    dismissAfterRun: 'auto' as const,
    run: (innerCtx: ExtensionContext) => switchDesktop(innerCtx, direction),
  };
}

export type DesktopDirection = 'next' | 'previous';

export const DESKTOP_SWITCH_FAILURE_MESSAGE =
  'Could not switch desktop. Turn on “Move left a space” and “Move right a space” in Settings › Keyboard › Keyboard Shortcuts › Mission Control, and allow Nevermind to control System Events in Privacy & Security › Automation.';

export function desktopSwitchScript(direction: DesktopDirection) {
  const keyCode =
    direction === 'next' ? RIGHT_ARROW_KEY_CODE : LEFT_ARROW_KEY_CODE;
  return `tell application "System Events" to key code ${keyCode} using control down`;
}

export function windowDesktopActions() {
  return [desktopAction('next'), desktopAction('previous')];
}
