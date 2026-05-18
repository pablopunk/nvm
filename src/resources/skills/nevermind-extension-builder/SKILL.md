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
5. Use `list_extensions` and `read_extension` when the request needs awareness of other installed extensions.
6. Call `list_capabilities` if the requested UI or OS operation is unclear.
7. Write one or more `.cjs` extension files with `write_extension`.
8. Validate changed files with `validate_extension`.
7. Tell the user the command title and aliases to search for.

Rules:

- Extensions export `module.exports = { id, title, commands }`.
- AI chats are builder/history sessions, not extension owners. Extensions are standalone files; a chat may create or touch multiple extension files.
- When tweaking an existing extension, keep the extension `id` and command `id`s exactly the same; IDs are persistent API and may be referenced by shortcuts.
- Commands should return `ctx.ui.*` views when they need UI.
- Use `rootItems(ctx)` for high-signal empty-query root palette contributions such as upcoming events or active status; keep root items few, stable, cached, and bounded because Nevermind owns ranking and limits.
- Prefer `ctx.ui.grid` for image/file galleries.
- Use `ctx.files.findImages`, `ctx.files.findVideos`, or `ctx.files.findMedia` for common galleries; use `ctx.files.find(roots, { extensions, kind, pattern, sortBy, order })` for custom filters.
- File helpers return objects with `path`, `name`, `displayPath`, `url`, `fileUrl`, `videoUrl`, `thumbnailUrl`, `kind`, `extension`, `mtime`, `mtimeMs`, `birthtime`, `birthtimeMs`, and `size`; use `{ sortBy: 'recent' }` for recently modified and `{ sortBy: 'added' }` for recently added/created.
- For grid videos, set `video: file.videoUrl` and `image: file.thumbnailUrl` so Nevermind can show a playable looping preview with a poster frame.
- Image thumbnails must use `file.url` from `ctx.files.findImages()` or `ctx.files.toFileUrl(path)`, never raw filesystem paths.
- Prefer `ctx.ui.form` for user input flows.
- Prefer `ctx.ui.chat` for conversational workflows.
- Prefer `ctx.ui.webview` for custom live/interactive browser UI; it runs sandboxed HTML/JS without Node access. Set `size: 'large'` when it needs a larger palette.
- Prefer declarative `ctx.actions.*` item actions over raw shell behavior.
- Use `primaryAction` for what Enter should do; all `actions` automatically appear under Cmd+K for each item.
- Use `ctx.navigation.push/replace/pop/run` as explicit return helpers from action handlers. Use `ctx.actions.push/replace/pop` for declarative view actions instead of inventing custom UI state; for media previews, use `ctx.actions.push('Preview', ctx.ui.preview(file), { shortcut: 'Command+Y' })` for in-app preview and `ctx.actions.quickLook(file.path)` for native macOS Quick Look when useful.
- Treat action shortcuts as local to the current view. Use command-level `globalShortcut` only for top-level commands that should run from anywhere; user-assigned global shortcuts take precedence.
- For Open With flows, never hardcode app names. Use `const apps = await ctx.files.openWithApps(file.path)` and create nested items whose primary action is `ctx.actions.openWith(file.path, app)`.
- Use `ctx.storage.memo(key, ttlMs, loader)` for expensive repeated work like indexing screenshots/media; use `ctx.storage.get/set/delete/clear` for persistent per-extension JSON state.
- Use `ctx.shell.exec(command, args, options)` or `ctx.shell.script(script, options)` for system automation when needed; keep commands focused, bounded, and show useful output/errors in native views.
- For grid views, choose `layout: 'wide'` for screenshots/videos, `layout: 'square'` for images/icons, or override with `aspectRatio`/`columns` when requested.
- Use `ctx.actions.run(title, async (ctx) => ...)` for script work triggered from UI; handlers may return another native view or another action to execute.
- Keep generated code small and readable.
- Nevermind catches thrown extension errors and renders an error view, so prefer throwing meaningful `Error` objects over swallowing failures unless the extension can recover or add context and rethrow.
- Do not use external dependencies.
- Do not write outside the generated extensions directory.
- Do not ask the user to edit files manually.
- If extension tools are unavailable, stop and report the tool failure instead of pasting code.
