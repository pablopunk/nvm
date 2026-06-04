# Extension API UI Fixtures

Nevermind's extension API is a product surface. Every API endpoint that renders host-owned UI must have a local dev-only fixture before it is considered supported.

AI agents testing or polishing this surface should use `.agents/skills/extensions-ui-test/SKILL.md`; it contains the full traversal, automation, and visual-review checklist.

## Rule

When adding or changing an extension API method that renders UI:

- Add or update a dummy extension under `src/fixtures/`.
- Make the fixture runnable from the single `Fixtures` root item in dev mode.
- Exercise realistic states: empty, populated, action panel, navigation, loading/error if applicable, and form submission where relevant.
- Keep fixtures dev-only. They must not be packaged or registered as shipped internal extensions.
- Verify with `mise exec pnpm -- pnpm test` and manually dogfood via `mise exec pnpm -- pnpm dev`.

## Current Fixture

`src/fixtures/ui-fixtures.ts` covers the current host-rendered UI surface:

- `ctx.ui.list`
- `ctx.ui.grid`
- `ctx.ui.preview`
- `ctx.ui.chat`
- `ctx.ui.form`
- `ctx.input.prompt`
- `ctx.ui.editor`
- `ctx.ui.progress`
- `ctx.ui.webview`
- `ctx.ui.camera`
- `ctx.ui.confirm`
- `ctx.ui.toast`

## Loading Model

The app loads `src/fixtures/` only in dev mode. Fixture extensions are registered for execution but hidden from normal root/search contributions; the only visible entry point is the dev-only `Fixtures` root item. Searching `fixtures` should return that single root item, and opening it should show fixture commands directly without another nesting layer. Fixtures are intentionally excluded from shipped product registration and from packaging.

## AI UI Testing Workflow

Use the project skill at `.agents/skills/extensions-ui-test/SKILL.md` for fixture dogfooding. At a minimum, an AI should:

1. Start the dev app with `mise exec pnpm -- pnpm dev`.
2. Connect automation through the dev CDP port: `agent-browser --cdp 9222 snapshot -i -c`.
3. Search for the `Fixtures` root item.
4. Open every fixture command.
5. Capture screenshots or accessibility snapshots where possible.
6. Check keyboard navigation, action panels, empty/loading/error states, and visual consistency.
7. Fix shared renderer/style/API primitives rather than patching fixture data.
8. Run `mise exec pnpm -- pnpm test`.

Prefer `agent-browser`'s Electron/dogfood workflows for automation when available.
