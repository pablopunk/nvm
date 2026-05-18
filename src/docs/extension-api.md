# Nevermind Extension API

Extensions are local `.cjs` modules loaded from Nevermind's user-data `extensions` directory. They are standalone app contributions, independent from the AI chats that created or edited them. They expose commands that appear in the main search results and can also contribute bounded items to the empty-query root palette. A command can execute work, return a declarative native view, or do both through item/action handlers.

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

Extensions can export `rootItems(ctx)` to contribute passive items to the root palette when there is no query. This is for ambient, high-signal information such as an upcoming calendar event, a currently running timer, or a recent status that deserves quick access.

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

Nevermind owns root ranking, rendering, limits, and failure isolation. Root contribution scores are capped by the host; extensions should return only a few useful items with stable IDs and bounded work. Root items use stale-while-revalidate semantics: the host returns the current cached snapshot for a palette render, refreshes stale/missing items in the background, and only shows refreshed items on a later search/open so the visible list does not shift under the user. Use `ctx.storage.memo` to cache expensive refreshes.

## Views

Commands can return native views. Nevermind owns keyboard navigation, filtering, Enter/default actions, Cmd+K item action panels, Escape/back navigation, nested view stacks, loading/empty/error rendering, accessories, and toasts.

Commands can return:

- `ctx.ui.list({ title, items, sections, selectedItemId, onSelectionChange, isLoading, emptyView, searchBarPlaceholder, searchAccessory, pagination })`; list items may include `accessories: [{ text }]` and `keywords`
- `ctx.ui.grid({ title, items, sections, selectedItemId, onSelectionChange, isLoading, emptyView, searchBarPlaceholder, searchAccessory, pagination, layout, aspectRatio, columns })` where `layout` can be `square`, `wide`, or `compact`
- `ctx.ui.preview({ title, content, image, video })` for text, image, and video previews; `ctx.ui.preview(file, { title, content })` builds a large media preview from an extension file object
- `ctx.ui.webview({ title, html, actions, size })` for live/interactive browser UI. Webviews run sandboxed HTML/JS without Node access and may use browser APIs like `navigator.mediaDevices` when the extension declares matching permissions such as `camera`. Use `size: 'large'` when the webview needs a larger palette.
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
- Actions can be grouped with `actionPanel: { sections: [{ title, actions }] }`; actions may include `submenu: { sections: [...] }` for nested action panels, `style: 'destructive'`, and `requiresConfirmation: true`.
- `ctx.navigation.push(view)`, `ctx.navigation.replace(view)`, `ctx.navigation.pop()`, and `ctx.navigation.run(action)` are the preferred explicit return helpers from action handlers.
- `ctx.actions.run(title, async (ctx) => { ... })` for custom work from a view action; it may return a `ctx.navigation.*` result, another view, another action to execute, `{ view }`, `{ action }`, or `{ toast }`.
- `ctx.actions.background(title, async (ctx) => { ... })` for fire-and-forget custom work that should dismiss the palette immediately and does not need follow-up UI. Command entries can set `background: true` or `dismissAfterRun: 'auto'` for the same command-level behavior.
- `ctx.actions.shellExec(title, command, args, options)` and `ctx.actions.shellScript(title, script, options)` for command actions that show structured output in a native preview view. These require confirmation by default.
- `ctx.storage.get/set/delete/clear/memo` for persistent per-extension JSON storage
- `ctx.extension.rename(title)` or `ctx.extension.rename({ title, subtitle, commandTitle, commandSubtitle })` to persistently rename the extension metadata shown in search results
- `ctx.ui.item/actions/empty/loading/error` helpers
- `ctx.cache`, `ctx.state`, `ctx.ai` placeholders

`ctx.desktop.files.find(roots, options)` supports `{ limit, depth, extensions, kind, pattern, sortBy, order }`, where `kind` can be `image`, `video`, or `media`, and `sortBy` can be `recent`/`modified`, `added`/`created`, `name`, or `size`. Convenience helpers `findImages`, `findVideos`, and `findMedia` call the same implementation. Returned files include `path`, `name`, `displayPath`, `url`, `fileUrl`, `videoUrl`, `thumbnailUrl`, `kind`, `extension`, `mtime`, `mtimeMs`, `birthtime`, `birthtimeMs`, and `size`. For grid videos, use `video: file.videoUrl` and `image: file.thumbnailUrl` to show a playable looping preview with a poster frame.

Use `await ctx.desktop.files.openWithApps(file.path)` to get installed apps that advertise support for that file type, then build an Open With nested view with `ctx.actions.openWith(file.path, app)`.

`ctx.storage` is scoped per extension file/identity, not per AI chat. `memo(key, ttlMs, loader)` caches expensive async work until the TTL expires:

```js
const files = await ctx.storage.memo('recent-media', 60_000, () =>
  ctx.desktop.files.findMedia(['~/Documents/CleanShot X'], { sortBy: 'recent', limit: 200 })
)
```

## Error handling

Extension command and action handlers may throw errors. Nevermind catches thrown errors and renders a native extension error view with the stack/message, so extensions should prefer throwing meaningful `Error` objects over swallowing failures or returning silent no-op states. Only catch errors when the extension can recover or add user-facing context before rethrowing.

Permissions are declared today and will become enforceable guardrails later.
