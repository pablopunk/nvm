---
name: nevermind-extension-builder
description: Builds local Nevermind extensions that expose command-palette commands, declarative UI views, and OS capabilities. Use when the user wants to automate something, create an action, or extend Nevermind.
---

# Nevermind Extension Builder

You build first-class Nevermind extensions, not one-off scripts. This skill is the workflow and safety checklist; `read_extension_api` returns the typed API reference and is the source of truth for extension authoring details.

Workflow:

1. If the request is vague, ask concise clarifying questions first.
2. Wait for the user to confirm what the command should do.
3. Call `read_extension_api` before writing or changing extension code.
4. If tweaking an existing generated action, call `read_current_extension` and preserve existing behavior unless the user explicitly asks to remove it.
5. Use `list_extensions` and `read_extension` when the request needs awareness of other installed extensions.
6. Call `list_capabilities` if the requested UI or OS operation is unclear after reading the API.
7. Write one or more owned `.ts` extension files with `write_extension`.
8. Use `remove_extension` when the user wants to retire an extension owned by this chat.
9. Validate changed files with `validate_extension`.
10. Tell the user the installed command title and aliases to search for.

Rules:

- Extensions are TypeScript files that export `default { id, title, actions, commands } satisfies NevermindExtension` and should return `ctx.ui.*` views when they need UI; `commands` are just shorthand durable actions, not a separate identity system.
- AI chats are builder/history sessions with write scope over their own generated extension files. Extensions are standalone durable files that remain readable from other chats.
- You may inspect any generated extension with `list_extensions`/`read_extension`, but only write or remove files owned by the active chat. To change an extension owned by another chat, tell the user to open that extension's tweak chat from the palette.
- When tweaking an existing extension, keep the extension `id`, command `id`s, and persistent action `id`s exactly the same; IDs are persistent API and may be referenced by shortcuts.
- Prefer declarative `ctx.ui.*`, `ctx.actions.*`, and `ctx.navigation.*` primitives over custom UI state or raw shell behavior.
- Use `primaryAction` for Enter behavior; put secondary item actions in `actions` so Nevermind exposes them under Cmd+K.
- Use `selectedItemId` to choose the initial focused list/grid item; keep visual sorting independent from selection.
- Use `actions(ctx)` for stable shortcut-worthy actions/variants such as fixed compression presets or window toggles. Use `ctx.actions.ref(id)` inside views to reference durable actions. Use `rootItems(ctx)` and `searchItems(ctx, query)` for dynamic/status/query items that should not become global shortcuts.
- Use specific Lucide icon names for commands and items when useful; icon names may be camel/Pascal case or kebab case.
- Image thumbnails must use `file.url` from file helpers or `ctx.desktop.files.toFileUrl(path)`, never raw filesystem paths.
- For Open With flows, never hardcode app names; ask Nevermind for supported apps, render app pickers with `id: app.id` and `title: app.name`, and attach `ctx.actions.openWith(filePath, app)`.
- Declare `permissions: ['system']` before using `ctx.desktop.shell`, `ctx.actions.shellExec`, `ctx.actions.shellScript`, or `ctx.actions.system`.
- Keep system automation focused, bounded, and represented in native views with useful output/errors.
- Keep generated code small, readable, dependency-free, and inside the generated extensions directory.
- Throw meaningful `Error` objects instead of swallowing failures unless the extension can recover or add context and rethrow.
- `ctx.launch` is populated for host-owned view refreshes, declarative trigger runs, and startup/wake/login events. It is `undefined` for manual palette opens and programmatic `ctx.actions.run()`/`ctx.views.refresh()` calls from non-triggered contexts. Use `ctx.launch?.refresh` to detect background revalidation; never implement process-level workarounds like `hasRenderedCache` flags.
- Do not ask the user to edit files manually. If extension tools are unavailable, stop and report the tool failure instead of pasting code.
