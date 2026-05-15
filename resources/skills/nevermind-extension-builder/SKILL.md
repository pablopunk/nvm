---
name: nevermind-extension-builder
description: Builds local Nevermind extensions that expose command-palette commands, declarative UI views, and OS capabilities. Use when the user wants to automate something, create an action, or extend Nevermind.
---

# Nevermind Extension Builder

You build Nevermind extensions, not one-off scripts.

Workflow:

1. If the request is vague, ask concise clarifying questions first.
2. Wait for the user to confirm what the command should do.
3. Call `read_extension_api`.
4. Call `list_capabilities` if the requested UI or OS operation is unclear.
5. Write a single `.cjs` extension with `write_extension` (this also replaces/activates the current generated action).
6. Validate it with `validate_extension`.
7. Tell the user the command title and aliases to search for.

Rules:

- Extensions export `module.exports = { id, title, commands }`.
- Commands should return `ctx.ui.*` views when they need UI.
- Prefer `ctx.ui.grid` for image/file galleries.
- Image thumbnails must use `file.url` from `ctx.files.findImages()` or `ctx.files.toFileUrl(path)`, never raw filesystem paths.
- Prefer `ctx.ui.form` for user input flows.
- Prefer `ctx.ui.chat` for conversational workflows.
- Prefer declarative `ctx.actions.*` item actions over raw shell behavior.
- Use `primaryAction` for what Enter should do; all `actions` automatically appear under Cmd+K for each item.
- Use `ctx.actions.push/replace/pop` for nested views instead of inventing custom UI state.
- Use `ctx.actions.run(title, async (ctx) => ...)` for script work triggered from UI; handlers may return another native view.
- Keep generated code small and readable.
- Do not use external dependencies.
- Do not write outside the generated extensions directory.
- Do not ask the user to edit files manually.
- If extension tools are unavailable, stop and report the tool failure instead of pasting code.
