import { showExtensionFeedback } from '../extensions/feedback.ts';
import type {
  ExtensionContext,
  ExtensionMenuBar,
  NevermindExtension,
} from '../resources/nevermind-extension-api';

const FOCUS_MINUTES = [15, 25, 50];

const state = {
  running: false,
  focusMinutes: 25,
  startedAt: 0,
  sound: true,
};

function remainingLabel() {
  const totalSeconds = state.focusMinutes * 60;
  const elapsed = state.running
    ? Math.floor((Date.now() - state.startedAt) / 1000)
    : 0;
  const left = Math.max(0, totalSeconds - elapsed);
  const minutes = String(Math.floor(left / 60)).padStart(2, '0');
  const seconds = String(left % 60).padStart(2, '0');
  return `${minutes}:${seconds}`;
}

function refreshAfter(ctx: ExtensionContext, update: () => void) {
  update();
  ctx.menuBar.refresh();
}

function focusMenuBar(ctx: ExtensionContext): ExtensionMenuBar {
  return {
    icon: state.running ? 'timer' : 'coffee',
    title: state.running ? remainingLabel() : undefined,
    tooltip: 'Menu Bar Fixture',
    sections: [
      {
        title: state.running ? 'Focusing' : 'Ready',
        items: [
          {
            id: 'toggle',
            title: state.running ? 'Stop Focus' : 'Start Focus',
            subtitle: `${state.focusMinutes} minute session`,
            icon: state.running ? 'pause' : 'play',
            shortcut: 'Command+Shift+F',
            action: ctx.actions.background('Toggle Focus', (inner) =>
              refreshAfter(inner, () => {
                state.running = !state.running;
                state.startedAt = Date.now();
              }),
            ),
          },
          {
            id: 'duration',
            title: 'Duration',
            icon: 'clock',
            items: FOCUS_MINUTES.map((minutes) => ({
              id: `minutes-${minutes}`,
              title: `${minutes} minutes`,
              checked: state.focusMinutes === minutes,
              action: ctx.actions.background(`${minutes} minutes`, (inner) =>
                refreshAfter(inner, () => {
                  state.focusMinutes = minutes;
                }),
              ),
            })),
          },
          {
            id: 'sound',
            title: 'Play Sound',
            checked: state.sound,
            action: ctx.actions.background('Toggle Sound', (inner) =>
              refreshAfter(inner, () => {
                state.sound = !state.sound;
              }),
            ),
          },
        ],
      },
      {
        title: 'Host actions',
        items: [
          {
            id: 'view',
            title: 'Open Session Log…',
            icon: 'list-todo',
            action: ctx.actions.run('Session Log', (inner) =>
              inner.ui.list({
                id: 'dev-ui-menubar-log',
                title: 'Session Log',
                items: [
                  {
                    id: 'state',
                    title: state.running ? 'Focusing' : 'Idle',
                    subtitle: `${state.focusMinutes} minute sessions · sound ${state.sound ? 'on' : 'off'}`,
                    icon: 'timer',
                  },
                ],
              }),
            ),
          },
          {
            id: 'copy',
            title: 'Copy Remaining Time',
            icon: 'clipboard',
            action: ctx.actions.background('Copy Remaining Time', (inner) => {
              showExtensionFeedback(
                inner,
                'Menu Bar Fixture',
                remainingLabel(),
              );
            }),
          },
          { id: 'disabled', title: 'Disabled Row', disabled: true },
        ],
      },
    ],
  };
}

const extension: NevermindExtension = {
  id: 'dev.ui-menubar-fixture',
  title: 'Menu Bar Fixture',
  subtitle: 'Dev-only Extension API menu bar fixture',
  menuBar: {
    triggers: [{ type: 'interval', every: '1s' }],
    render: focusMenuBar,
  },
};

export default extension;
