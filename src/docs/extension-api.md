# Nevermind Extension API

Extensions are local `.cjs` modules loaded from Nevermind's user-data `extensions` directory. They are standalone app contributions with durable files independent from AI chat history, while AI builder chats keep a write scope over the extension files they created or touched. AI builder chats may inspect any generated extension for context, but only the chat that owns an extension file can overwrite it. Extensions expose commands that appear in the main search results and can also contribute bounded items to the empty-query root palette. A command can execute work, return a declarative native view, or do both through item/action handlers.

```js
module.exports = {
  id: 'my.images',
  title: 'My Images',
  commands: [
    {
      id: 'image-grid',
      title: 'Show Image Grid',
      subtitle: 'Browse recent images',
      aliases: ['pics', 'photos'],
      icon: 'grid',
      permissions: ['files:read', 'ui:grid'],
      async run(ctx) {
        const images = await ctx.desktop.files.findImages(['~/Downloads', '~/Desktop'], { limit: 48 })
        return ctx.ui.grid({
          title: 'Images',
          items: images.map((file) => ({
            id: file.path,
            title: file.name,
            subtitle: file.displayPath,
            image: file.url, // thumbnail-safe display URL; Nevermind drags actions with a path as the original file
            primaryAction: ctx.actions.copyImage(file.path),
            actions: [
              ctx.actions.copyImage(file.path),
              ctx.actions.copyText(file.path, 'Copy path'),
              ctx.actions.revealPath(file.path),
              ctx.actions.quickLook(file.path),
              ctx.actions.push('Preview', ctx.ui.preview({
                title: file.name,
                content: `# ${file.name}\n\n${file.displayPath}`,
              })),
            ],
          })),
        })
      },
    },
  ],
}
```

## Root contributions

Extensions can export `rootItems(ctx)` to contribute passive items to the root palette when there is no query, and `searchItems(ctx, query)` to contribute bounded query-aware root results. Root items are for ambient, high-signal information such as an upcoming calendar event, a currently running timer, or a recent status that deserves quick access.

```js
module.exports = {
  id: 'my.calendar',
  title: 'Calendar',
  async rootItems(ctx) {
    const event = await ctx.storage.memo('next-event', 60_000, async () => null)
    if (!event) return []
    return [{
      id: `next-event-${event.id}`,
      title: event.title,
      subtitle: `Starts ${event.startsIn}`,
      icon: 'calendar',
      score: 80,
      primaryAction: ctx.actions.openUrl(event.url, 'Open event'),
    }]
  },
  commands: [],
}
```

Nevermind owns root ranking, rendering, limits, and failure isolation. Root/search contribution scores are capped by the host; extensions should return only a few useful items with stable IDs and bounded work. Root items use stale-while-revalidate semantics: the host returns the current cached snapshot for a palette render, refreshes stale/missing items in the background, and only shows refreshed items on a later search/open so the visible list does not shift under the user. Query-aware `searchItems(ctx, query)` contributions run under the same timeout and per-extension caps. Use `ctx.storage.memo` to cache expensive refreshes.

## Views

Commands can return native views. Nevermind owns keyboard navigation, filtering, Enter/default actions, Cmd+K item action panels, Escape/back navigation, nested view stacks, loading/empty/error rendering, accessories, icons, and toasts. Command, root item, and list item `icon` values accept any Lucide icon name in camel/Pascal case or kebab case, such as `mic`, `volume-2`, `audio-lines`, `calendar`, or `folder`; older aliases like `restart`, `grid`, and `sparkles` remain supported.

Commands can return:

- `ctx.ui.list({ title, items, sections, selectedItemId, onSelectionChange, isLoading, emptyView, searchBarPlaceholder, searchAccessory, pagination })`; list items may include `accessories: [{ text }]` and `keywords`
- `ctx.ui.grid({ title, items, sections, selectedItemId, onSelectionChange, isLoading, emptyView, searchBarPlaceholder, searchAccessory, pagination, refresh, layout, aspectRatio, columns })` where `layout` can be `square`, `wide`, or `compact`
- `ctx.ui.preview({ title, content, image, video, actions, actionPanelVisibility })` for text, image, and video previews; `ctx.ui.preview(file, { title, content })` builds a large media preview from an extension file object
- `ctx.ui.webview({ title, html, actions, size, actionPanelVisibility })` for live/interactive browser UI. Webviews run sandboxed HTML/JS without Node access and may use browser APIs like `navigator.mediaDevices` when the extension declares matching permissions such as `camera`. Use `size: 'large'` when the webview needs a larger palette.
- `ctx.ui.chat({ title, messages })`
- `ctx.ui.form({ title, fields })`
- `ctx.ui.progress({ title, steps })`

## Context capabilities

Current `ctx` namespaces:

- `ctx.desktop.clipboard.readText/writeText/readImage/writeImage/readFiles/read/write`
- `ctx.desktop.selection.text/files/read` for current desktop selection such as selected text or selected files
- `ctx.desktop.apps.frontmost/launch`
- `ctx.desktop.files.find/findImages/findVideos/findMedia/openWithApps/open/reveal/preview/readText/toFileUrl`
- `ctx.desktop.shell.openExternal`, `ctx.desktop.shell.exec(command, args, options)`, `ctx.desktop.shell.script(script, options)`, `ctx.desktop.shell.appleScript(script, options)`, and `ctx.desktop.shell.which(command)` for controlled system work. Shell helpers return `{ stdout, stderr, exitCode }` and default to a 30s timeout.
- `ctx.actions.openPath/revealPath/quickLook/openWith/openUrl/copyText/pasteText/copyImage/trash` (optional final `{ shortcut: 'Command+Y' }` for local shortcuts). `quickLook` opens native macOS Quick Look and reports an error on other platforms. `trash` is destructive and requires confirmation by default.
- Shortcuts have two scopes: action shortcuts inside views are local by default; command-level shortcuts are global when declared as `globalShortcut` or `{ shortcut, shortcutScope: 'global' }`. User-assigned global shortcuts always win over extension defaults.
- `ctx.actions.push(title, view, { shortcut })`, `ctx.actions.replace(title, view, { shortcut })`, `ctx.actions.pop(title, { shortcut })` for nested native navigation
- Actions can be grouped with `actionPanel: { sections: [{ title, actions }] }`; actions may include `submenu: { sections: [...] }` for nested action panels, `style: 'destructive'`, and `requiresConfirmation: true`. Views and items can set `actionPanelVisibility: 'hidden'` when actions should remain available to default actions/local shortcuts but not be rendered as an action menu or inline preview/webview panel.
- `ctx.navigation.push(view)`, `ctx.navigation.replace(view)`, `ctx.navigation.pop()`, and `ctx.navigation.run(action)` are the preferred explicit return helpers from action handlers.
- `ctx.actions.run(title, async (ctx) => { ... })` for custom work from a view action; it may return a `ctx.navigation.*` result, another view, another action to execute, `{ view }`, `{ action }`, `{ toast }`, or `{ patch: { mode: 'patch' | 'replace' | 'prepend', items: [{ id, ...fields }] } }` to update the current view in place without rebuilding it. Views can set `refresh: { intervalMs, action, mode }` so the host owns polling while the view is open; use `replace` for fresh sorted snapshots and `prepend` for newly discovered items.
- `ctx.actions.background(title, async (ctx) => { ... })` for fire-and-forget custom work that should dismiss the palette immediately and does not need follow-up UI. Command entries can set `background: true` or `dismissAfterRun: 'auto'` for the same command-level behavior.
- `ctx.actions.shellExec(title, command, args, options)` and `ctx.actions.shellScript(title, script, options)` for command actions that show structured output in a native preview view. These require confirmation by default.
- `ctx.storage.get/set/delete/clear/memo/memoStale` for persistent per-extension JSON storage
- `ctx.settings.definitions/get/set/toggle` for host-owned app settings exposed to first-party extension workflows
- `ctx.actions.toggleSetting(settingId, title)` and `ctx.actions.setPaletteShortcut(title)` for declarative settings actions
- `ctx.ai.ask(prompt, options)` for a one-shot AI call that returns text. Options may include `{ system }`.
- `ctx.ai.session(id, options)` for a per-extension conversational AI session. The returned session supports `ask(prompt)` and `reset()`. Session ids are scoped to the extension, and options may include `{ system }`.
- `ctx.extension.rename(title)` or `ctx.extension.rename({ title, subtitle, commandTitle, commandSubtitle })` to persistently rename the extension metadata shown in search results
- `ctx.ui.item/actions/empty/loading/error` helpers
- `ctx.cache` and `ctx.state` placeholders

`ctx.desktop.files.find(roots, options)` supports `{ limit, depth, extensions, kind, pattern, sortBy, order }`, where `kind` can be `image`, `video`, or `media`, and `sortBy` can be `recent`/`modified`, `added`, `created`, `name`, or `size`. `recent`/`modified` sorts by filesystem modification time (`mtimeMs`), which can be older than the download time when files preserve original metadata. `added` sorts by macOS Finder/Spotlight Date Added (`dateAddedMs`) with filesystem creation time as a fallback, and is usually the right choice for “newest files”, Downloads, screenshots, and mixed media galleries. `created` sorts by filesystem creation time (`birthtimeMs`). Convenience helpers `findImages`, `findVideos`, and `findMedia` call the same implementation. Returned files include `path`, `name`, `displayPath`, `url`, `fileUrl`, `videoUrl`, `thumbnailUrl`, `kind`, `extension`, `mtime`, `mtimeMs`, `birthtime`, `birthtimeMs`, `dateAdded`, `dateAddedMs`, and `size`. For grid videos, use `video: file.videoUrl` and `image: file.thumbnailUrl` to show a playable looping preview with a poster frame.

Use `await ctx.desktop.files.openWithApps(file.path)` to get installed apps that advertise support for that file type, then build an Open With nested view with `ctx.actions.openWith(file.path, app)`.

`ctx.storage` is scoped per extension file/identity, not per AI chat. `memo(key, ttlMs, loader)` caches expensive async work until the TTL expires. `memoStale(key, ttlMs, staleTtlMs, loader)` returns a stale cached value immediately while refreshing in the background, and only waits for `loader` when there is no usable cached value:

```js
const files = await ctx.storage.memoStale('recent-media', 60_000, 24 * 60 * 60_000, () =>
  ctx.desktop.files.findMedia(['~/Downloads', '~/Desktop'], { sortBy: 'added', limit: 200 })
)
```

## AI builder write scope

The extension-building tools intentionally separate read access from write ownership. Builder chats can use `list_extensions` and `read_extension` to understand related generated extensions, but `write_extension` can only replace extension files already owned by the active chat. A single chat may own multiple related extension files. To change an extension owned by another chat, open that extension's tweak chat from the palette instead of overwriting it from the current conversation.

## Error handling

Extension command and action handlers may throw errors. Nevermind catches thrown errors and renders a native extension error view with the stack/message, so extensions should prefer throwing meaningful `Error` objects over swallowing failures or returning silent no-op states. Only catch errors when the extension can recover or add user-facing context before rethrowing.

Permissions are declared today and will become enforceable guardrails later.
