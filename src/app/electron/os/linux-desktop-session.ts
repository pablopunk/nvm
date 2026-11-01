import { app } from 'electron';

function isWaylandSession() {
  return (
    String(process.env.XDG_SESSION_TYPE || '').toLowerCase() === 'wayland' ||
    Boolean(process.env.WAYLAND_DISPLAY)
  );
}

/** Must run before app readiness: Chromium reads these switches at startup. */
export function prepareLinuxDesktopSession(desktopName: string) {
  if (process.platform !== 'linux') return;
  app.setDesktopName(desktopName);
  if (!isWaylandSession()) return;
  app.commandLine.appendSwitch('enable-features', 'GlobalShortcutsPortal');
  app.commandLine.appendSwitch('disable-features', 'WaylandFractionalScaleV1');
}
