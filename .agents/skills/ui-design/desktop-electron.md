# Desktop & Electron Conventions

How to avoid the Electron uncanny valley and feel native — primarily macOS, with cross-platform notes.

## Fonts

```css
html {
  font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text",
               ui-sans-serif, system-ui, "Segoe UI", Roboto, sans-serif;
  font-feature-settings: "ss01", "cv11"; /* SF Pro stylistic sets */
}
code, kbd, samp {
  font-family: ui-monospace, "SF Mono", Menlo, Consolas, monospace;
}
```

Never ship a webfont for app chrome — system fonts give the OS's rendering and weight quirks for free.

## Cursors

```css
button, [role="button"], .clickable { cursor: default; }
a[href], a[href] * { cursor: pointer; }
```

Native macOS apps don't use the hand cursor on buttons. Only true links to outside the app get `pointer`.

## Text selection

```css
:root { user-select: none; -webkit-user-select: none; }
input, textarea, [contenteditable], .selectable { user-select: text; -webkit-user-select: text; }
```

Selecting button labels by accident is a top tell that an app is Electron.

## Drag regions

```css
.titlebar { -webkit-app-region: drag; }
.titlebar button, .titlebar input { -webkit-app-region: no-drag; }
```

The whole window chrome should be draggable except interactive elements.

## Window behavior

- Wait for `ready-to-show` before showing the BrowserWindow. Never show a white flash.
- Match system appearance: `nativeTheme.shouldUseDarkColors` and `prefers-color-scheme`.
- For a palette window: `frame: false`, `transparent: true`, `vibrancy: 'hud'` (macOS), `visualEffectState: 'active'`.
- `titleBarStyle: 'hiddenInset'` for main windows that need traffic lights but no title.
- Restore window position and size between launches. Never re-center on every open unless it's the palette.

## Menu bar

- Provide a full menu bar even on a tray app. Empty menus look broken.
- macOS expects: App · File · Edit · View · Window · Help. Add app-specific menus between View and Window.
- **Preferences** under the app menu (macOS) or File menu (Windows/Linux), bound to `⌘,` on macOS.
- Every menu item that has a shortcut must display it. Use Electron `accelerator: 'CommandOrControl+S'` to get the right key per OS.

## Tray / status bar

- The tray icon must be a monochrome template image on macOS (`Template.png`) so it inverts in dark menu bar.
- Tray click opens the palette directly, not a tray menu, if the palette is the primary surface.
- Right-click on tray → small menu with Quit, Preferences, About.

## Notifications

- Use the OS notification API, not in-app toasts, for events that happen while the app is in the background.
- In-app toasts only for foreground confirmation (saved, copied, etc.) and auto-dismiss in 2–3s.

## Cross-platform pitfalls

- `⌘` ≠ `Ctrl`. Use `CommandOrControl` accelerators and render the key conditionally in UI: `isMac ? '⌘' : 'Ctrl'`.
- Window controls are on the right on Windows/Linux, left on macOS. If you draw a custom title bar, mirror accordingly.
- Scrollbars overlay on macOS, take space on Windows. Avoid layouts that depend on scrollbar width.
- Right-click → context menu is universal; on macOS also support `Ctrl+click`.

## Performance tells

- Animations stutter at 30fps on Electron more than on native. Keep transitions short (<300ms) and on `transform`/`opacity` only.
- Don't animate `box-shadow`, `width`, `height`, `top/left`. Use `transform: scale/translate` and `opacity`.
- Lazy-load heavy panels; the palette itself should be in memory from first launch.

## macOS HIG essentials

- **Sheets** for modal flows attached to a window. **Panels** for floating helpers. **Popovers** for transient context.
- Buttons: default action is blue and rightmost. Cancel is to its left. Destructive action is far left and styled red.
- Avoid emoji-as-icon in chrome — SF Symbols (or icon set matching SF Symbol style) feel native.
- Respect Reduce Motion and Increase Contrast system settings.
