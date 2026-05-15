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
4. If tweaking an existing generated action, call `read_current_extension` and preserve existing behavior unless the user explicitly asks to remove it.
5. Call `list_capabilities` if the requested UI or OS operation is unclear.
6. Write a single `.cjs` extension with `write_extension` (this also replaces/activates the current generated action).
6. Validate it with `validate_extension`.
7. Tell the user the command title and aliases to search for.

Rules:

- Extensions export `module.exports = { id, title, commands }`.
- When tweaking an existing extension, keep the extension `id` and command `id`s exactly the same; IDs are persistent API and may be referenced by shortcuts.
- Commands should return `ctx.ui.*` views when they need UI.
- Prefer `ctx.ui.grid` for image/file galleries.
- Use `ctx.files.findImages`, `ctx.files.findVideos`, or `ctx.files.findMedia` for common galleries; use `ctx.files.find(roots, { extensions, kind, pattern, sortBy, order })` for custom filters.
- File helpers return objects with `path`, `name`, `displayPath`, `url`, `fileUrl`, `videoUrl`, `thumbnailUrl`, `kind`, `extension`, `mtime`, `mtimeMs`, `birthtime`, `birthtimeMs`, and `size`; use `{ sortBy: 'recent' }` for recently modified and `{ sortBy: 'added' }` for recently added/created.
- For grid videos, set `video: file.videoUrl` and `image: file.thumbnailUrl` so Nevermind can show a playable looping preview with a poster frame.
- Image thumbnails must use `file.url` from `ctx.files.findImages()` or `ctx.files.toFileUrl(path)`, never raw filesystem paths.
- Prefer `ctx.ui.form` for user input flows.
- Prefer `ctx.ui.chat` for conversational workflows.
- Prefer declarative `ctx.actions.*` item actions over raw shell behavior.
- Use `primaryAction` for what Enter should do; all `actions` automatically appear under Cmd+K for each item.
- Use `ctx.actions.push/replace/pop` for nested views instead of inventing custom UI state; for media previews, use `ctx.actions.push('Preview', ctx.ui.preview(file), { shortcut: 'Command+Y' })` for in-app preview and `ctx.actions.quickLook(file.path)` for native macOS Quick Look when useful.
- Use `ctx.storage.memo(key, ttlMs, loader)` for expensive repeated work like indexing screenshots/media; use `ctx.storage.get/set/delete/clear` for persistent per-extension JSON state.
- For grid views, choose `layout: 'wide'` for screenshots/videos, `layout: 'square'` for images/icons, or override with `aspectRatio`/`columns` when requested.
- Use `ctx.actions.run(title, async (ctx) => ...)` for script work triggered from UI; handlers may return another native view.
- Keep generated code small and readable.
- Do not use external dependencies.
- Do not write outside the generated extensions directory.
- Do not ask the user to edit files manually.
- If extension tools are unavailable, stop and report the tool failure instead of pasting code.
