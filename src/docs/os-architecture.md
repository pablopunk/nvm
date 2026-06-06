# OS architecture

Nevermind should treat operating-system behavior as product capabilities, not as incidental platform branches. Features should describe what the user wants to do, ask whether that intent is supported on the current system, and present the best palette-native experience for that system.

## Intention

- Keep the app cross-platform by default and OS-specific only by explicit choice.
- Make every desktop integration explainable as a user-facing capability.
- Let each OS choose the right interaction for the same intent: a toggle, a selector, a nested view, a direct action, or no visible item.
- Keep unsupported capabilities out of normal discovery surfaces.
- Preserve the command-k model: OS choices, warnings, confirmations, and settings belong inside the palette.
- Keep extension failures palette-safe when generated commands assume desktop behavior that is unavailable on the current OS.

## Capability mindset

A capability is a user intent the operating system may help fulfill. It should be named after the outcome, not after the mechanism used to achieve it.

Good capability framing:

- Open an installed app.
- Preview a file.
- Reveal a file in the system file manager.
- Paste into the frontmost app.
- Lock the computer.
- Open system settings.
- Configure a global shortcut.
- Start the app after sign-in.
- Change an OS appearance setting.

Bad capability framing:

- Run AppleScript.
- Call a specific command-line tool.
- Parse one desktop file format.
- Use a specific macOS, Linux, or Windows API as the product concept.

Implementation mechanisms can change. The product capability should remain stable.

## Product rules

- If a capability is unsupported, do not show it in the root command list.
- If an unsupported action is reached from old state, a shortcut, or an extension, fail inside the palette with a clear message.
- Prefer omission over disabled clutter unless the unavailable state teaches the user something useful.
- Use OS-neutral labels in shared UI. OS-specific words such as Finder, Quick Look, Spotlight, System Settings, or Start Menu should come from the OS capability that owns that experience.
- Different platforms may expose different UI for the same intent. Do not force every OS into the macOS interaction model.
- Linux desktop-environment differences should be modeled as capability differences, not as renderer special cases.
- Windows support should be explicit. A missing Windows implementation is not the same as a deliberately unsupported capability.

## Engineering rules

- OS checks belong in the OS capability layer, packaging code, or one-time startup selection only.
- Shared command, renderer, extension, and model code should ask for capability behavior instead of branching by platform.
- First-party OS primitives should be generic enough for built-in commands and extensions to share.
- Capability results should be palette-ready: labels, descriptions, warnings, and available actions should already reflect the current OS.
- Extension APIs should stay focused on user intents and palette actions, not private platform internals.
- Defensive execution is still required because saved shortcuts, old state, and generated extensions may reference actions that are no longer supported.

## Desired state

All desktop integrations are owned by OS capabilities. The rest of the app describes user intent and receives palette-ready behavior for the current system.

- App discovery, app launching, and app watching are one capability family.
- Built-in system commands such as lock, sleep, restart, settings, and quit-related affordances are capability-driven.
- File actions such as preview, reveal, open with, trash, drag, thumbnails, and selected files use OS-owned labels and availability.
- Frontmost-app interactions such as paste are offered only when the current system can perform them reliably.
- Camera access for host-owned extension views is capability-gated, uses OS privacy prompts where required, and must include platform packaging metadata such as macOS camera usage descriptions.
- Palette window behavior such as focus, taskbar/dock visibility, all-workspaces behavior, and desktop-window-manager quirks is not spread through generic window lifecycle code.
- Shortcut behavior such as default accelerators, conflicts, formatting, and keyboard-settings guidance reflects the current OS.
- Login startup behavior appears only on platforms that can own it and should keep the palette hidden until the user invokes it.
- Update availability and packaging-dependent messaging are presented as platform capabilities, not as generic promises.
- Extensions that invoke unavailable OS-dependent actions fail safely inside the palette.
- Shared UI contains no hard-coded OS wording for behavior owned by a capability.

## Documentation rules

When adding or changing an OS capability, document only:

- the user intent;
- when it should appear;
- what the user should experience on supported systems;
- how unsupported systems should behave;
- any product-level platform caveats future contributors must preserve.

Do not document file names, functions, command invocations, API calls, or other implementation details here. The code owns implementation; docs own intent, guidelines, and rules.
