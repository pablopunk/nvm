# Selected-text capture must own the clipboard until Copy finishes

## Report

Slack text was visibly selected, but **Fix Selected Text with AI** displayed “Select text to fix”; **Nevermind OS Permissions** displayed Accessibility as allowed.

## Code findings

- The clipboard fallback returned a polling promise from inside `try` without awaiting it. Its `finally` restored the original clipboard before asynchronous capture finished. A delayed Copy could therefore yield the original clipboard text or an empty result. Existing tests supplied copied text immediately and used synchronous clipboard reads.
- Native Copy errors, failed helper launches, missing source apps, and focused-palette guards could return the same empty result. Permission denial during Copy was not propagated to the command.
- The permissions list checked Electron's trust status, although a separate native process reads text and posts Copy events. Native Accessibility API denial could also be discarded as a missing attribute.

These findings explain failure paths in the code; retained local logs did not contain the reported attempt, so its exact path remains unconfirmed.

## Required behavior

- Hold the clipboard snapshot until capture resolves or rejects, then restore it before returning to the caller.
- Do not mistake a delayed Copy, a blocked native operation, or a lost source app for proof that the user selected nothing.
- Check permission in the process that performs the native operation, including keyboard-event access for Copy, and use that check in the permissions list.
- Record capture method, length, and native failure stage without recording selected text.

## Regression coverage and remaining validation

Coverage added for delayed asynchronous Copy, empty and unrelated original clipboards, clipboard restoration after failures, concurrent readers, native error propagation, host/helper permission differences, and permission-error feedback.

Tests, typechecks, native compilation, and app launches are intentionally skipped locally under repository policy. A scoped CI workflow now compiles both native architectures on changes to the helper. CI has not run for this repair.

The packaged macOS journey remains outstanding: select text in Slack, run the command, confirm replacement and clipboard restoration, then confirm blocked access is reported as a permission error and is not shown as allowed in the permissions list.

## Checks run

Formatting was applied with:

```sh
mise exec -- pnpm format src/app/electron/selected-text.ts src/app/electron/selected-text.test.ts src/app/electron/macos-selected-text.ts src/app/electron/macos-selected-text.test.ts src/app/electron/main.ts src/app/electron/os.ts src/app/extensions/permissions.ts src/app/extensions/permissions.test.ts src/app/extensions/ai-commands.ts src/app/extensions/ai-commands.test.ts src/app/resources/nevermind-extension-api.d.ts
mise exec -- pnpm format src/app/electron/macos-selected-text.ts src/app/electron/macos-selected-text.test.ts src/app/resources/nevermind-extension-api.d.ts
```

These checks passed:

```sh
git diff --check
mise exec -- pnpm check src/app/electron/selected-text.ts src/app/electron/selected-text.test.ts src/app/electron/macos-selected-text.ts src/app/electron/macos-selected-text.test.ts src/app/electron/main.ts src/app/electron/os.ts src/app/extensions/permissions.ts src/app/extensions/permissions.test.ts src/app/extensions/ai-commands.ts src/app/extensions/ai-commands.test.ts src/app/resources/nevermind-extension-api.d.ts
```

The initial `mise exec -- pnpm check:changed 299e43e` attempt processed no files before commit; it is not lint evidence.

Keywords: Slack, Select text to fix, Accessibility allowed, clipboard fallback, return await, finally, selected-text helper, CGPreflightPostEventAccess, apiDisabled.
