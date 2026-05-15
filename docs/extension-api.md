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
- `ctx.ui.grid({ title, items })`
- `ctx.ui.detail({ title, content })`
- `ctx.ui.chat({ title, messages })`
- `ctx.ui.form({ title, fields })`
- `ctx.ui.progress({ title, steps })`

## Context capabilities

Initial `ctx` namespaces:

- `ctx.clipboard.readText/writeText/readImage/writeImage`
- `ctx.files.find/findImages/selectedInFinder/open/readText/toFileUrl`
- `ctx.actions.openPath/revealPath/openUrl/copyText/copyImage`
- `ctx.actions.push(title, view)`, `ctx.actions.replace(title, view)`, `ctx.actions.pop(title)` for nested native navigation
- `ctx.actions.run(title, async (ctx) => { ... })` for script work from a view action; it may return another view
- `ctx.apps.launch/frontmost`
- `ctx.shell.openExternal`
- `ctx.ui.item/actions/empty/loading/error` helpers
- `ctx.cache`, `ctx.state`, `ctx.ai` placeholders

Permissions are declared today and will become enforceable guardrails later.
