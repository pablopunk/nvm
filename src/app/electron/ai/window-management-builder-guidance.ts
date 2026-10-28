export const WINDOW_GUIDANCE = `Window management (capability 'desktop.window-management'; launching apps also needs 'desktop.apps'): guard first. Check that ctx.desktop.windowManagement is defined and ctx.system.capabilities.has('window-management') is true; when either is missing, return a friendly ctx.ui.preview message instead of throwing.
Facet methods: apply(spec, options?) lays out the focused window of the app that was frontmost before Nevermind opened, or the app a shortcut targeted. windows({ appId?, appName?, title? }) lists windows of running apps; appName and title are case-insensitive substring matches, minimized or other-Space windows may be missing, and access denied returns an empty list. waitForWindow({ appId?, appName?, title? }, { timeoutMs? }) polls until the first match or null (default 5000 ms, max 30000). applyTo(handle, spec, options?) lays out one window from windows() or waitForWindow() and works from any invocation, including views and rows. focus(handle) brings a window and its app forward. displays() returns each display's id, isPrimary, frame and workArea in global points with a top-left origin.
Handles: every window has handle: { id }. Pass window.handle, never construct one, and never keep one across sessions. When a result reports reason 'unknown-window', call windows() or waitForWindow() again.
Specs (kind): fraction (x, y, width, height as 0..1 of the work area, optional cycleGroup); center; maximize-width; maximize-height; scaled-center (widthFraction, heightFraction, optional maxWidth and maxHeight in points); edge (left, right, top or bottom); resize-step (direction larger or smaller, step 0.01..0.25); display (next or previous); restore; toggle-full-screen; exact (x, y, width, height, each { value, unit: 'points' | 'percent' }; omitted values keep the current value; the result stays inside the work area).
Options for apply and applyTo: outerGap { top, right, bottom, left } in whole points 0..40 (default 8 each), innerGap 0..40 (default 4), applyGapToMaximize (default true), plus cycle, cycleOrder, displayOrder and displayMoveSizing. Pass only what the user asked for.
Results: { ok: true, outcome: 'exact' | 'partial', changed, frame } or { ok: false, reason, message }. Check ok, show message to the user, and do not treat failures as exceptions.
Identifiers: appId is the stable platform app identifier (the bundle identifier on macOS) and is not the same as the id returned by ctx.desktop.apps.search(name); match a found app by its name with appName.
Example: put the user's browser on the left at 60% and the user's terminal on the right at 40%. Find each app by name; when it has no window, launch it and wait for its window.
async function windowFor(ctx, app) {
  if (!app) return null;
  const open = (await ctx.desktop.windowManagement.windows()).find((window) => window.appName === app.name);
  if (open) return open;
  ctx.desktop.apps.launch(app.path);
  return ctx.desktop.windowManagement.waitForWindow({ appName: app.name });
}
async function placeBrowserAndTerminal(ctx) {
  const browser = await windowFor(ctx, ctx.desktop.apps.search('Safari')[0]);
  const terminal = await windowFor(ctx, ctx.desktop.apps.search('Terminal')[0]);
  if (!browser || !terminal) return ctx.ui.preview({ title: 'Window not found', content: 'Open both apps and try again.' });
  const wm = ctx.desktop.windowManagement;
  const left = await wm.applyTo(browser.handle, { kind: 'exact', x: { value: 0, unit: 'percent' }, y: { value: 0, unit: 'percent' }, width: { value: 60, unit: 'percent' }, height: { value: 100, unit: 'percent' } });
  const right = await wm.applyTo(terminal.handle, { kind: 'exact', x: { value: 60, unit: 'percent' }, y: { value: 0, unit: 'percent' }, width: { value: 40, unit: 'percent' }, height: { value: 100, unit: 'percent' } });
  return ctx.ui.preview({ title: 'Windows placed', content: [left, right].map((result) => result.ok ? 'Placed.' : result.message).join('\\n') });
}`;
