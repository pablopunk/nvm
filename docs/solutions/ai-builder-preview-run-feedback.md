# AI Builder Preview Run Feedback

## Problem

The AI extension builder announced a generated command was installed and working while its preview already showed `... failed` (`TypeError: text.trim is not a function`). Fix with AI then repaired it.

## Root cause

`validate_extension` and `write_extension` only typecheck and load the extension. The first command runs later in the palette renderer, so its outcome never reached the agent, which reported success from a load-only check.

## Rejected approach

Running the first command on the host inside `write_extension` executed side-effecting commands for inactive chats, ran them twice when the renderer did not reuse the result, lost late results behind a timeout while holding the write lock, and fired loader patches before the renderer rendered the view.

## Fix

The renderer stays the only executor: it runs the first command once, only for the active preview. The host observes that run through the normal action IPC path (`createPreviewRunObserver`) and records how it ended. `write_extension` returns immediately and says the command is unverified. `read_preview_run` waits briefly, then reports ran, failed with the underlying error, or not run. It never runs anything, and a late result stays readable.

## Limits

Only the first command's own run is checked. Inactive or closed previews, runs still in progress, async view loaders, and later interactions stay unverified, and the agent must say so.

## Verification

Focused unit tests cover the observer, the action IPC hook, and the tool contracts. Confirm in a packaged build by asking the builder for a command that throws on first run and checking that the agent repairs it before reporting success, and that an inactive chat reports "unverified" without running the command.
