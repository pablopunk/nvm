# AI Builder Preview Run Feedback

## Problem

The AI extension builder announced a generated command was installed and working while its preview already showed `... failed` (`TypeError: text.trim is not a function`). Fix with AI then repaired it.

## Root cause

`validate_extension` and `write_extension` only typecheck and load the extension. The first command ran later in the palette renderer, so its outcome never reached the agent, which reported success from a load-only check.

## Fix

`write_extension` runs the preview's first command on the host through the same action path the renderer used and returns the outcome. A failure makes the tool error, with the underlying error text, so the agent must repair and rewrite. The renderer reuses the host result instead of running the command a second time, and the preview still shows the error view with Fix with AI.

## Verification

Focused unit tests cover the runner, the tool contract, and the renderer reuse. Confirm in a packaged build by asking the builder for a command that throws on first run and checking that the agent repairs it before reporting success.
