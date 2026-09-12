# macOS Accessibility Writes Must Be Verified

## Problem or symptoms

"Fix Selected Text with AI" showed "Fixing Text" and then never changed the selected text. The indicator disappeared with no error and the host logged no failure. Users saw this in Slack on `v0.17.x`.

## Context

- Command: `nevermind.ai-commands` action `fix-selected-text-with-ai` in `src/app/extensions/ai-commands.ts`.
- Host path: `ctx.desktop.selection.replaceText` -> `replaceSelectedText` in `src/app/electron/os.ts` -> `src/app/resources/macos-selected-text.swift`.
- Clipboard fallback: `ctx.actions.pasteText` with `concealed`, `restoreClipboard`, and `expectedFrontmostAppId`.
- The unverified accessibility write came from the "replace selected text without clipboard" change and shipped in `v0.17.x`.

## Root cause

`AXUIElementSetAttributeValue(kAXSelectedTextAttribute)` reports success when macOS delivers the request, not when the app applies it. Chromium-based apps such as Slack return success and ignore the write. The extension treated exit code 0 as a completed replacement, returned early, and never ran the paste fallback.

## What did not work

- Retrying the AI call and rejecting non-proofreading responses only addressed model output quality.
- Hardening palette focus restoration only addressed reads, not writes.

## Diagnosis

- `.tmp/dev.log` showed `extension.root-item.handler` and `ipc.view-action:execute` ending within about 1 ms of each other. Clipboard snapshot, write, and paste work would have taken longer, so the fallback never ran.
- No `selected-text.replace.failed` entry appeared, so the helper exited 0 or 3.
- `hidePalette.restoreFocus` logged `restored: true`, ruling out the frontmost-app guard.

## Fix

- The command always pastes corrected text through `ctx.actions.pasteText`, and skips replacement when the AI returns the text unchanged.
- The Swift helper polls the element's `AXValue` after the write and reports the no-selection exit code unless the value reflects the replacement.
- `replaceSelectedText` logs `selected-text.replace.notApplied` at debug level when the helper reports an unapplied write.
- `docs/os-architecture.md` records that selected-text writes report success only after the target app's text reflects the change.

## Verification

- `ai-commands.test.ts` fakes `selection.replaceText` returning true and asserts the command still returns the paste action, so it cannot silently depend on the accessibility write again.
- Local biome checks and `swiftc -parse` pass; the packaged app build compiles the helper.
- Dev-app journey confirmed the corrected text appears in place in Slack.

## Notes for future searches

Keywords: Fix Selected Text with AI, Slack, Chromium, Electron, AXSelectedText, AXUIElementSetAttributeValue, kAXValueAttribute, accessibility write, silent no-op, replaceText, selected-text.replace.notApplied, Fixing Text, nevermind.ai-commands, clipboard paste fallback.
