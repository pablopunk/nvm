# OS architecture

Nevermind should treat operating-system behavior as capabilities, not scattered platform checks. A feature should ask the current OS layer what it can do, how it should be presented, and which action shape executes it; unsupported capabilities should disappear or degrade into an explicit, palette-native error only when invoked from old state or extension output.

## Goals

- Keep command, extension, and renderer code platform-neutral by default.
- Make platform differences discoverable in one place before adding a desktop feature.
- Let each OS expose a different UI contract for the same intent when needed: a toggle on macOS, a selector on Linux, or no item on Windows.
- Prefer generic first-party capabilities over helpers tailored to one command or generated extension.
- Preserve the command-k product model: decisions, confirmations, and configuration stay inside palette views and action panels.

## Current platform seams to extract

- Application discovery and launch: macOS `.app` roots and `open`, Linux `.desktop` files, Windows Start Menu shortcuts.
- File affordances: Quick Look, reveal labels, open-with app matching, thumbnails, drag icons, trash, and selected Finder items.
- System commands: lock, sleep, restart, OS settings, keyboard settings, paste into the frontmost app.
- Window policy: panel/accessory behavior, all-workspaces visibility, taskbar/dock presence, focus and blur handling.
- Shortcuts: Spotlight conflict detection, accelerator labels, global shortcut registration behavior.
- Updates and packaging: macOS zip updates, Linux AppImage updates, Windows support decisions.
- Extension runtime capabilities: AppleScript and native actions that should be advertised as conditional instead of hard-coded.

## Proposed shape

Introduce a main-process `os` module with a small stable interface and platform implementations selected once at startup.

```ts
type OsCapability =
  | 'applications'
  | 'app-icons'
  | 'quick-look'
  | 'open-with'
  | 'selected-files'
  | 'frontmost-paste'
  | 'keyboard-settings'
  | 'system-settings'
  | 'power-actions'
  | 'window-panel-policy'
  | 'applescript'

type DesktopOs = {
  id: 'macos' | 'linux' | 'windows'
  label: string
  capabilities: Set<OsCapability>
  builtins(): Action[]
  executeBuiltin(action: Action): Promise<ActionResult | void>
  applications: ApplicationProvider
  files: FileProvider
  window: WindowPolicy
  shortcuts: ShortcutPolicy
  extensionRuntime: ExtensionRuntimePolicy
}
```

Providers should return palette-ready data, not leak raw OS details to the renderer. If a capability needs a different interaction model per OS, expose that as actions or views from the provider rather than branching in React.

## Migration plan

1. Add an `src/electron/os/` directory with the interface, shared fallbacks, and `macos`, `linux`, and `windows` implementations.
2. Move application scanning, app launching, and app watching first; this is the largest visible cross-platform surface and already has three implementations in `main.ts`.
3. Move built-in system actions next, so `builtInActions` is generated from capabilities instead of `process.platform` checks.
4. Move file affordances: Quick Look, open-with, reveal wording, selected Finder files, and frontmost paste.
5. Move window policy out of `palette-window.ts`, keeping only generic palette lifecycle there.
6. Move shortcut policy: default accelerator choices, Spotlight conflict handling, display caveats, and keyboard-settings deep links.
7. Expose a read-only capability snapshot to extensions so generated extensions can check support before offering native actions.
8. Delete renderer-facing OS copy such as “macOS Quick Look” and “Finder” from shared models unless it comes from the OS layer.

## Rules for future OS-dependent features

- Do not add new `process.platform` checks outside the OS layer unless the file is itself an OS implementation or packaging bootstrap.
- Do not show unsupported commands in the root list; unsupported action execution should be defensive only.
- If Linux behavior depends on a desktop environment, model it as a detected capability or provider variant, not as renderer branching.
- Prefer capability names based on user intent, not implementation technology.
- Document new capabilities here when adding them, including the intended UX on each supported OS.
