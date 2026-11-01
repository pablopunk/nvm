import type { ExtensionContext } from '../../resources/nevermind-extension-api';

export function progressView(ctx: ExtensionContext) {
  return ctx.ui.progress({
    id: 'dev-ui-progress',
    title: 'Dev UI · Progress',
    status: 'Rendering fixture preview…',
    value: 2,
    total: 3,
    steps: [
      { title: 'Design API primitive', status: 'Done' },
      { title: 'Render fixture', status: 'In progress' },
      { title: 'Dogfood locally', status: 'Next' },
    ],
  });
}

const SETUP_STEPS = [
  {
    title: 'Describe the project',
    guidance: 'Say what it does and who it is for.',
  },
  {
    title: 'Choose a workspace',
    guidance: 'Pick a folder and the tools to use.',
  },
  {
    title: 'Plan the first milestone',
    guidance: 'Name one small deliverable.',
  },
];

function setupStepStatus(index: number, current: number) {
  if (index < current) return 'Complete';
  return index === current ? 'In progress' : 'Upcoming';
}

/** Steps without `value`/`total`: the host counts completed steps and shows the next action below them. */
export function progressStepsView(ctx: ExtensionContext, current = 0) {
  const finished = current >= SETUP_STEPS.length;
  const view = ctx.ui.progress({
    id: 'dev-ui-progress-steps',
    title: 'Dev UI · Progress Steps',
    label: finished
      ? 'Setup complete'
      : `Step ${current + 1} of ${SETUP_STEPS.length}`,
    status: finished
      ? 'Every step is complete.'
      : SETUP_STEPS[current].guidance,
    steps: SETUP_STEPS.map((step, index) => ({
      title: step.title,
      status: setupStepStatus(index, current),
    })),
  });
  const nextTitle =
    current === SETUP_STEPS.length - 1 ? 'Finish setup' : 'Continue';
  return finished
    ? view
    : {
        ...view,
        actions: [
          ctx.actions.run(nextTitle, function advanceSetupStep(actionCtx) {
            return actionCtx.navigation.replace(
              progressStepsView(actionCtx, current + 1),
            );
          }),
        ],
      };
}

export function webviewView(ctx: ExtensionContext) {
  return ctx.ui.webview({
    id: 'dev-ui-webview',
    title: 'Dev UI · Webview',
    html: '<style>:root{color-scheme:light dark;--surface-webview:Canvas;--text-webview:CanvasText}body{margin:0;background:var(--surface-webview);color:var(--text-webview);font-family:system-ui}main{min-height:100vh;padding:24px}</style><main><h1>Sandboxed Webview Fixture</h1><p>No Node access. Use only when host primitives do not fit.</p><button>Focusable button</button></main>',
  });
}

export function cameraView(ctx: ExtensionContext) {
  return ctx.ui.camera({
    id: 'dev-ui-camera',
    title: 'Dev UI · Camera',
    showDeviceSwitcher: true,
    controls: true,
  });
}

export function mirroredCameraView(ctx: ExtensionContext) {
  return ctx.ui.camera({
    id: 'dev-ui-camera-mirrored',
    title: 'Dev UI · Mirrored Camera',
    mirrored: true,
    framing: 'contain',
    showDeviceSwitcher: true,
  });
}
