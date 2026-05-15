# Nevermind Extension API

Extensions are local `.cjs` modules loaded from Nevermind's user-data `extensions` directory. They expose commands that appear in the main search results. A command can execute work, return a declarative native view, or do both through item/action handlers. AI-generated extensions are idempotent per chat/action: writing again replaces the same generated extension instead of creating a duplicate.

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
        const images = await ctx.files.findImages(['~/Downloads', '~/Desktop'], { limit: 48 })
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
              ctx.actions.push('Show Details', ctx.ui.detail({
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

## Views

Commands can return native views. Nevermind owns keyboard navigation, filtering, Enter/default actions, Cmd+K item action panels, Escape/back navigation, nested view stacks, loading/empty/error rendering, and toasts.

Commands can return:

- `ctx.ui.list({ title, items })`
- `ctx.ui.grid({ title, items, layout, aspectRatio, columns })` where `layout` can be `square`, `wide`, or `compact`
- `ctx.ui.detail({ title, content, image, video })`
- `ctx.ui.chat({ title, messages })`
- `ctx.ui.form({ title, fields })`
- `ctx.ui.progress({ title, steps })`
- `ctx.ui.preview(file, { title, content })` for a built-in image/video preview detail view

## Context capabilities

Current `ctx` namespaces:

- `ctx.clipboard.readText/writeText/readImage/writeImage`
- `ctx.files.find/findImages/findVideos/findMedia/selectedInFinder/open/readText/toFileUrl`
- `ctx.actions.openPath/revealPath/quickLook/openUrl/copyText/copyImage` (optional final `{ shortcut: 'Command+Y' }` for local shortcuts). `quickLook` opens native macOS Quick Look and reports an error on other platforms.
- `ctx.actions.push(title, view, { shortcut })`, `ctx.actions.replace(title, view, { shortcut })`, `ctx.actions.pop(title, { shortcut })` for nested native navigation
- `ctx.actions.run(title, async (ctx) => { ... })` for script work from a view action; it may return another view
- `ctx.apps.launch/frontmost`
- `ctx.shell.openExternal`
- `ctx.storage.get/set/delete/clear/memo` for persistent per-extension JSON storage
- `ctx.ui.item/actions/empty/loading/error` helpers
- `ctx.cache`, `ctx.state`, `ctx.ai` placeholders

`ctx.files.find(roots, options)` supports `{ limit, depth, extensions, kind, pattern, sortBy, order }`, where `kind` can be `image`, `video`, or `media`, and `sortBy` can be `recent`/`modified`, `added`/`created`, `name`, or `size`. Convenience helpers `findImages`, `findVideos`, and `findMedia` call the same implementation. Returned files include `path`, `name`, `displayPath`, `url`, `fileUrl`, `videoUrl`, `thumbnailUrl`, `kind`, `extension`, `mtime`, `mtimeMs`, `birthtime`, `birthtimeMs`, and `size`. For grid videos, use `video: file.videoUrl` and `image: file.thumbnailUrl` to show a playable looping preview with a poster frame.

`ctx.storage` is scoped per extension, using the generated chat identity when available so tweaks keep the same storage. `memo(key, ttlMs, loader)` caches expensive async work until the TTL expires:

```js
const files = await ctx.storage.memo('recent-media', 60_000, () =>
  ctx.files.findMedia(['~/Documents/CleanShot X'], { sortBy: 'recent', limit: 200 })
)
```

Permissions are declared today and will become enforceable guardrails later.
