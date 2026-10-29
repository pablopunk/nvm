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
