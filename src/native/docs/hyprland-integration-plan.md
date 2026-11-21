# Hyprland integration: shortcuts, action dispatch and window appearance

Status: H1-H5 implemented in repository code (see "Implementation"); the packaged-app journey on a real Hyprland session is outstanding. Live configuration changes are not authorized by this plan. Companion to WP1.11, WP3.8, WP4.1c and WP5.4 in `../PLAN.md`.

## Intended experience

Users set shortcuts and window appearance inside Command-K. On Hyprland, Nevermind offers one-time setup of a Nevermind-owned compositor config module. Subsequent changes update that module, preserve unrelated user configuration, and report whether the compositor actually accepted them. A modular config does not necessarily load our file: setup must establish and verify an include explicitly.

A shortcut can open the palette or run a named action, including when Nevermind is not running. Palette and floating notes can appear without app or compositor decorations while retaining their intended focus, workspace, movement, resize and dismissal behavior. Ordinary notes remain ordinary windows, not layer surfaces.

Scope is repository work followed by an explicitly authorized integration journey on a Hyprland test session and packaged app. No production service or backend change is needed. This document authorizes no editing of a user's compositor config, reload, app launch or release.

## Architecture decisions

- Portable Nevermind settings remain the source of truth. Compositor files are generated output, not an additional settings store.
- Global shortcuts and window appearance are separate intent-named capabilities. They share session/version detection, config format detection, a managed module, include setup, preview/apply/undo, atomic writes, conflict reporting, validation and reload.
- Keep platform decisions below feature and extension layers. Built-ins and extensions use the same declarative host primitives; extensions do not receive arbitrary compositor config or shell-writing access.
- Probe asynchronously and cache the last known state. First paint, typing and action execution must not wait for detection, CLI calls or decoration enrichment.
- Scope rules to stable, verified Nevermind application identity and owned window roles. Never target the currently active window, every application, or an assumed development identity.
- Prefer persistent rules for first-map appearance and restart-safe bindings. Runtime per-window IPC is an optional adapter capability only after the installed version proves it supports the required operation.

## Work packages

### H1: establish the supported boundary

Recheck the latest integration head before implementation, including the extension `WindowOptions.Chrome` declaration and host mapping, shortcut registration, instance forwarding, packaged executable identity and layer namespace. Read prior root causes in `docs/solutions/`.

Detect the running Hyprland version and actual config format; verify supported arguments against the installed CLI before selecting commands or emitting syntax. Current upstream documentation uses Lua, while older versions use `.conf`. Do not ship unversioned rule snippets or infer compatibility from the compositor name alone. Verify exact application and window-role matching, including development versus packaged identities and note versus palette surfaces.

Deliver a supported-version/config-format table, explicit unsupported states and fixtures for the contracts the adapters consume. Ambiguous or read-only configuration must produce a palette explanation and a copyable/manual setup path rather than guessed edits.

### H2: launch and run actions from the CLI

Add a public `run <action-id>` command to the packaged Nevermind executable; confirm the installed command name rather than assuming `nevermind` or `nvm` is on PATH. Resolve stable registered action IDs through the existing command/extension execution path, retaining permission checks and confirmation requirements. Include a way to discover or copy an action's CLI invocation in the palette.

If Nevermind is running, forward the request to that instance. Otherwise start one instance, wait for the requested action's registry to be ready, and dispatch once. Preserve the frontmost external application as the action target; do not show or focus the palette unless the action requires UI. A palette-open action explicitly opens it. Requests must not run twice during concurrent launches or instance forwarding.

Define bounded readiness and forwarding timeouts, help output, and truthful exit/error semantics: distinguish rejected requests, accepted interactive actions and completed noninteractive actions. Unknown, removed or unavailable actions must fail clearly without running a substitute. Use a user-scoped local transport; do not expose a network listener or treat raw input as executable code.

The current `nvm://` route handles authentication. General action URLs are not required for this milestone: unlike a local CLI, links can originate from websites. A future action URL needs an explicit exposure/consent policy rather than inheriting unrestricted CLI execution.

### H3: shared compositor configuration lifecycle

Offer setup, preview, apply, repair, undo and removal inside Command-K using result rows and action panels. Use native pickers when selecting a nonstandard config resource is necessary. Preview must identify exact affected files and the include change before applying it.

Own one generated module with separately owned shortcut and appearance sections. Establish the include once with user approval, or provide the exact include for manual setup; verify it is loaded. Respect custom entrypoints, symlinks and config ownership. Never assume a conventional modular directory or wildcard include exists.

Use atomic writes and snapshots of the affected owned content. Detect changes since preview and do not overwrite concurrent manual edits. Validate using commands supported by the detected version, reload only after valid output, and preserve the previous working state on write, validation or reload failure. Reconcile desired settings and applied compositor state; retain pending intent and show failure rather than claiming success. Undo must restore the previous managed state without reverting unrelated edits.

Detect shortcut and appearance conflicts across effective matching configuration where inspectable. Do not assume appending a rule wins. If effective state cannot be established, report it as unverified. Removal cleans up only Nevermind-owned content and the include installed by setup, preserving user additions. Update, removal, repair and undo must be idempotent.

### H4: Hyprland shortcut registration

Keep shortcut selection and canonical accelerators in Nevermind. Generate version-appropriate bindings that invoke the installed executable's public action CLI, so a binding can also start Nevermind. Preserve proper argument escaping and stable action identity; never concatenate untrusted action text into shell commands.

Retain portal registration as a separate supported mechanism where appropriate, with explicit ownership so an accelerator does not execute an action twice. Hyprland's portal `global` dispatcher requires a live registered target; it does not by itself establish the cold-start action journey. If used, verify actual app and shortcut IDs rather than guessing them.

Cover palette summon, built-in actions and extension actions through the same shortcut capability. A compositor binding conflict, missing executable, missing include or failed reload must be visible in the palette. Keep focus capture, recorder suspension, action shortcuts and dismissal behavior as one contract.

### H5: decoration policy and declarative windows

Centralize app-owned window appearance policy and route it through the declarative host primitive. Migrate built-in floating notes through that primitive before exposing missing extension functionality. Recheck and complete `WindowOptions.Chrome` mapping on the current head; update the API source of truth and add dev fixtures for any UI API change.

Apply the policy at window construction. Audit `Frameless` plus `DisableShadow` for true `chrome:none`; `TitleBarHidden` alone is insufficient evidence. Preserve opaque content: transparency is not a prerequisite for removing GTK client-side decorations. Preserve move, resize, close, focus restoration, keyboard navigation and workspace placement contracts.

Use version-supported Hyprland rules for compositor decoration/effect preferences, investigating `border_size`, `decorate`, `rounding`, `no_shadow` and `no_blur` rather than assuming all versions accept them. Distinguish ordinary window rules from layer effects. Verify the palette's actual layer-shell availability and namespace; do not apply ordinary-window matching to an unverified layer surface. Missing protocol/library support and conflicting user rules must produce truthful capability status.

### H6: follow-on compositor adapters

Niri and Umbriel can reuse detection, managed output and palette setup, but do not block Hyprland delivery. Establish separate syntax, identity, reload and conflict contracts before enabling either adapter.

Niri's explicit `on` decoration rules can override `off` across matching rules, so file order alone cannot resolve conflicts. Its inspected public IPC provided no arbitrary decoration-rule setter. Umbriel exposes decoration config and some runtime effect actions, but a full runtime override is unproven. Do not generalize Hyprland's mechanisms to either compositor.

## Implementation

- H2: `internal/shortcuts/actioncli` (command line, protocol, exactly-once forwarding) over `internal/platform/instance` (private per-user socket, background start). Exit statuses: 0 completed/started/opened, 2 usage, 3 rejected, 4 failed, 5 unreachable or result lost. `palette` opens the palette and `palette-toggle` toggles it as the summon key does. Keyboard Shortcuts rows copy an action's command line.
- H1, H3, H4: `internal/platform/hyprland` owns detection, the version table (package doc), rendering, include setup and the apply/undo/remove lifecycle; `internal/shortcuts/compositor` owns desired state, kept state and portal ownership; the Hyprland Integration page appears in Command-K only in a Hyprland session.
- H5: `Chrome: "none"` maps to no frame and no shadow (no GTK client-side border on Wayland); a hidden title bar keeps the OS window buttons. Floating notes use it. Compositor rules for Nevermind's ordinary windows are a user choice on the Hyprland page; the palette is a layer surface and gets no window rule.

Assumptions still to confirm on a real session: `Hyprland --verify-config --config <file>`, the `hyprctl binds -j` fields (`modmask`, `key`, `arg`), hyprlang `on`/`off` rule values, Lua `pcall(require, "nevermind")` beside the entry, `exec_cmd` running `sh -c`, and the Wayland app ID being the executable's basename (or the packaged command name).

## Evidence and open questions

The supplied decoration investigation inspected `t3/investigate-mygo-docs` at `e772f69a18c812faaaa3f52a824d304dede65650` and upstream MyGo v0.7.4. These are source-level findings, not proof of current packaged behavior:

- Palette construction already uses Panel, Frameless and Transparent. A layer-shell path avoids ordinary window borders, but layer effects and library/protocol availability remain to be verified.
- Floating notes used TitleBarHidden without DisableShadow, leaving GTK client-side border/shadow on Wayland. Linux `SetHasShadow` is a no-op in the inspected MyGo source; construction-time policy is required.
- MyGo Linux app identity derives from the executable basename (expected `nvm` in development). Packaged identity remains unverified. Layer namespace derives from the backend name or argv basename fallback; inspect it in the actual session.
- `WindowOptions.Chrome` existed in the inspected branch; an independent-window host mapping was not found then. Recheck the current integration head.
- xdg-decoration negotiates decoration ownership; it is not a universal focus-ring/shadow/effect suppression API. The inspected hyprland-surface-v1 protocol exposes opacity and visible-region, without a decoration-off request.
- CLI action dispatch is proposed work; the checked-out native entrypoint has internal child/OCR/build-flags modes and auth deep links, not a public action runner.

Sources for implementation-time revalidation:

- [MyGo windows](https://mygo.egoist.dev/docs/windows) and [MyGo v0.7.4 Linux window source](https://raw.githubusercontent.com/egoist/mygo/v0.7.4/internal/linux/window.go).
- [Hyprland window rules](https://github.com/hyprwm/hyprland-wiki/blob/main/content/configuring/core/rules/window-rules.md), [global shortcuts](https://wiki.hypr.land/configuring/core/binds/globals/) and [hyprland-surface-v1](https://github.com/hyprwm/hyprland-protocols/blob/main/protocols/hyprland-surface-v1.xml).
- [Niri window rules](https://github.com/niri-wm/niri/blob/main/docs/wiki/Configuration%3A-Window-Rules.md) and [Umbriel window rules](https://docs.noctalia.dev/umbriel/window-rules/).

## Acceptance and verification

Repository checks belong in CI. Add meaningful contract coverage for installed CLI/version variants, action dispatch and instance races, config lifecycle failure paths, argument escaping, ownership and identity matching. Add declarative window fixtures and interaction coverage for focus, resize and dismissal. Run the native package, layer and file-size checks specified by the corresponding WPs in CI; do not run local tests, builds, app launches or rig journeys without explicit local-verification authorization.

The separately outstanding packaged-app journey must run on a named real Hyprland test session and record the exact package/commit, compositor version, config format, executable identity and layer namespace. Acceptance requires:

- The actual configured key opens the palette after a compositor/session restart. With Nevermind stopped, the key launches it and opens the palette; repeated keys do not spawn duplicate instances.
- A configured action key runs the intended built-in or extension action exactly once, both cold and warm, preserving the external target/focus and showing UI only when needed. Removed actions and unavailable extensions fail visibly.
- Palette and notes have the requested decoration-free appearance; note resize/move/close and palette navigation, action panels, shortcuts, focus restoration and dismissal still work. Record ordinary-window and layer-surface evidence separately.
- Unrelated user config survives setup, updates, removal and undo. Missing include, conflicting rules/bindings, concurrent edits, invalid generated output and reload failures remain visible and recoverable.
- Reapplying, removing and undoing changes are idempotent. Both shortcut and appearance settings survive restart and are actually effective, not merely present in a generated file.

A green CI result proves repository behavior only. Keep the Hyprland journey outstanding until observed on the packaged artifact; record any supported-version or hardware/session coverage deferred. No implementation or runtime validation has been performed as part of writing this plan.
