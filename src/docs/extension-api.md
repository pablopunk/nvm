# Nevermind Extension API

This is the canonical guideline/API reference for extension authors. It intentionally holds authoring rules, contracts, and examples that would overflow `AGENTS.md` or the builder skill, not implementation details of the host internals.

Extensions are local `.ts` modules loaded from Nevermind's user-data `extensions` directory with Electron/Node's native TypeScript type stripping. They are standalone app contributions with durable files independent from AI chat history, while AI builder chats keep a write scope over the extension files they created or touched. AI builder chats may inspect any generated extension for context, but only the chat that owns an extension file can overwrite it. Extensions expose commands that appear in the main search results and can also contribute bounded items to the empty-query root palette. A command can execute work, return a declarative native view, or do both through item/action handlers.

Use erasable TypeScript syntax only: types, interfaces, generics, and `satisfies` are supported; runtime-only TypeScript features such as enums, decorators, namespaces with values, and parameter properties are not supported unless they are rewritten as plain JavaScript. Generated extensions can import host types with `import type { NevermindExtension } from './nevermind-extension-api'`.

```ts
import type { NevermindExtension } from './nevermind-extension-api'

export default {
  id: 'my.images',
  title: 'My Images',
  permissions: ['desktop.files'],
  commands: [
    {
      id: 'image-grid',
      title: 'Show Image Grid',
      subtitle: 'Browse recent images',
      aliases: ['pics', 'photos'],
      icon: 'grid',
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
} satisfies NevermindExtension
```

## Root contributions

Extensions can export `rootItems(ctx)` to contribute passive items to the root palette when there is no query, and `searchItems(ctx, query)` to contribute bounded query-aware root results. Commands already appear in root/search, so providers must not return command launchers or collection entry points; use providers only for distinct child items, ambient status, or query-specific results. Root items are for ambient, high-signal information such as an upcoming calendar event, a currently running timer, or a recent status that deserves quick access.

```ts
import type { NevermindExtension } from './nevermind-extension-api'

export default {
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
} satisfies NevermindExtension
```

Nevermind owns root ranking, rendering, limits, and failure isolation. Root/search contribution scores are capped by the host; extensions should return only a few useful items with stable IDs and bounded work. Root items use stale-while-revalidate semantics: the host returns the current cached snapshot for a palette render, refreshes stale/missing items in the background, and only shows refreshed items on a later search/open so the visible list does not shift under the user. Query-aware `searchItems(ctx, query)` contributions run under the same timeout and per-extension caps. Use `ctx.storage.memo` to cache expensive refreshes.

Items in root, list, and grid views may include `appearance: { foreground }` to visually differentiate item families without custom rendering. `foreground` is a muted named color: `yellow`, `blue`, `purple`, `green`, `red`, `orange`, or `pink`. The host validates these names and ignores unsupported values.

## Views

Commands can return native views. Nevermind owns keyboard navigation, filtering, Enter/default actions, Cmd+K item action panels, Escape/back navigation, nested view stacks, loading/empty/error rendering, accessories, icons, and toasts. Command, root item, and list item `icon` values accept any Lucide icon name in camel/Pascal case or kebab case, such as `mic`, `volume-2`, `audio-lines`, `calendar`, or `folder`; older aliases like `restart`, `grid`, and `sparkles` remain supported.

Commands can return:

- `ctx.ui.list({ title, items, sections, selectedItemId, onSelectionChange, isLoading, emptyView, searchBarPlaceholder, searchAccessory, pagination })`; list items may include `accessories: [{ text }]` and `keywords`
- `ctx.ui.grid({ title, items, sections, selectedItemId, onSelectionChange, isLoading, emptyView, searchBarPlaceholder, searchAccessory, pagination, refresh, layout, aspectRatio, columns })` where `layout` can be `square`, `wide`, or `compact`
- List and grid items may include `appearance: { foreground: 'yellow' }` using the same muted foreground color names as root items.
- `ctx.ui.preview({ title, content, image, video, actions, actionPanelVisibility })` for text, image, and video previews; `ctx.ui.preview(file, { title, content })` builds a large media preview from an extension file object
- `ctx.ui.camera({ title, deviceId, showDeviceSwitcher, muted, controls, actions, actionPanel, size, actionPanelVisibility })` for a host-owned live camera surface. Use this instead of custom HTML for webcam previews; Nevermind owns permission, rendering, sizing, camera switching, and stream lifecycle. Declare `camera` permission. Desktop multi-camera switching is host-owned; use `deviceId` only when you already have a stable browser camera id. Camera views support normal view actions and shortcuts; use `ctx.actions.camera.switchDevice('Switch Camera', { shortcut: 'Command+P' })`, `nextDevice`, `previousDevice`, `toggleMuted`, or `toggleControls` to bind host camera controls without owning the stream.
- `ctx.ui.webview({ title, html, actions, size, actionPanelVisibility })` for advanced custom live/interactive browser UI when no host-owned primitive fits. Webviews run sandboxed HTML/JS without Node access. Use `size: 'large'` when the webview needs a larger palette.
- `ctx.ui.chat({ title, messages })`
- `ctx.ui.form({ title, fields })`
- `ctx.ui.progress({ title, label, steps, id, value, total })` returns a host-rendered progress view. Pass `label` for a single active step, or `steps: [{ title, status }]` for a multi-step indicator. Optional `id`, `value`, and `total` are forwarded for downstream progress tracking.

## Declarative UI lifecycle primitives

The renderer owns confirmation, preview, and progress surfaces; extensions request them declaratively through `ctx.ui.*` so they never need renderer-only state branches:

- `ctx.ui.confirm({ title?, message?, confirmLabel?, cancelLabel?, destructive?, onConfirm })` wraps an inner action so the host renders a confirmation step before executing it. `onConfirm` may be any declarative action (e.g. `ctx.actions.run(...)`, a native action, or another `ctx.ui.*` result). Pass `destructive: true` to render the destructive style; `message`, `confirmLabel`, and `cancelLabel` customize the confirmation panel copy.
- `ctx.ui.preview({ kind, title?, text?, imageDataUrl?, imagePath?, videoUrl?, filePath?, thumbnailUrl?, clipboardType? })` opens the host-owned inline preview pane. `kind` is one of `'clipboard' | 'image' | 'video' | 'file' | 'text'`. For a full preview as a stacked view (with markdown, image, or video content), keep using the existing `ctx.ui.preview({ title, content, image, video })` view form or `ctx.ui.preview(file, { title, content })`.
- `ctx.ui.toast({ message, tone? })` returns an action-result toast (`{ toast: { message, tone } }`). Equivalent to returning `{ toast: { ... } }` directly; use it when composing results from `ctx.actions.run`.

## Context capabilities

Current `ctx` namespaces:

- `ctx.desktop.clipboard.readText/writeText/readImage/writeImage/readFiles/read/write`
- `ctx.desktop.selection.text/files/read` for current desktop selection such as selected text or selected files
- `ctx.desktop.apps.frontmost/launch`
- `ctx.desktop.files.find/findImages/findVideos/findMedia/openWithApps/open/reveal/preview/readText/toFileUrl`
- `ctx.desktop.shell.openExternal`, `ctx.desktop.shell.exec(command, args, options)`, `ctx.desktop.shell.script(script, options)`, `ctx.desktop.shell.appleScript(script, options)`, and `ctx.desktop.shell.which(command)` for controlled system work. Shell helpers return `{ stdout, stderr, exitCode }` and default to a 30s timeout.
- `ctx.actions.openPath/revealPath/quickLook/openWith/openUrl/copyText/pasteText/copyImage/trash` (optional final `{ shortcut: 'Command+Y' }` or similar for local shortcuts). `quickLook` opens native macOS Quick Look and reports an error on other platforms; it does not declare a shortcut unless the extension explicitly passes one. `trash` is destructive and requires confirmation by default.
- Shortcuts have two scopes: action shortcuts inside views are local by default; command-level shortcuts are global when declared as `globalShortcut` or `{ shortcut, shortcutScope: 'global' }`. User-assigned global shortcuts always win over extension defaults.
- `ctx.actions.push(title, view, { shortcut })`, `ctx.actions.replace(title, view, { shortcut })`, `ctx.actions.pop(title, { shortcut })` for nested native navigation
- Actions can be grouped with `actionPanel: { sections: [{ title, actions }] }`; actions may include `submenu: { sections: [...] }` for nested action panels, `style: 'destructive'`, and `requiresConfirmation: true`. Views and items can set `actionPanelVisibility: 'menu'` to hide inline action chrome while still making actions available through Cmd+K and local shortcuts, or `actionPanelVisibility: 'hidden'` when actions should remain shortcut-only and not be rendered as an action menu or inline preview/webview panel.
- `ctx.navigation.push(view)`, `ctx.navigation.replace(view)`, `ctx.navigation.pop()`, and `ctx.navigation.run(action)` are the preferred explicit return helpers from action handlers.
- `ctx.actions.run(title, async (ctx) => { ... })` for custom work from a view action; it may return a `ctx.navigation.*` result, another view, another action to execute, `{ view }`, `{ action }`, `{ toast }`, or `{ patch: { mode: 'patch' | 'replace' | 'prepend' | 'append', items: [{ id, ...fields }], removeItemIds, isLoading, selectedItemId } }` to update the current view in place without rebuilding it. Prefer event/delta-driven patches over interval refreshes for native/internal views: keep the visible list stable, and emit targeted patches (prepend new items, patch changed rows, list removed ids in `removeItemIds`) when a known action mutates state. `refresh: { intervalMs, action, mode }` is supported for extension compatibility but should not be used as the reference pattern. `isLoading` should reflect explicit work in progress (saving, downloading, toggling), not passive background revalidation.
- `ctx.actions.background(title, async (ctx) => { ... })` for fire-and-forget custom work that should dismiss the palette immediately and does not need follow-up UI. Command entries can set `background: true` or `dismissAfterRun: 'auto'` for the same command-level behavior.

`ctx.actions.*` helpers only create declarative actions; calling `ctx.actions.run(...)` does not execute the handler. For open, stateful views whose backing data may change while visible, return a fast cached initial snapshot and set `refresh: { intervalMs, action, mode }` so the host owns the background work. Use `replace` for sorted snapshots such as screenshots, settings, shortcuts, or status lists, and `prepend` for append-only feeds. The host only polls while the view is active, avoids overlapping refreshes, and ignores stale refresh results after navigation.
- `ctx.actions.shellExec(title, command, args, options)` and `ctx.actions.shellScript(title, script, options)` for command actions that show structured output in a native preview view. These require confirmation by default.
- `ctx.storage.get/set/delete/clear/memo/memoStale` for persistent per-extension JSON storage
- `ctx.settings.definitions/get/set/toggle` for host-owned app settings exposed to first-party extension workflows
- `ctx.logs.debug/info/warn/error(message, data?)` writes extension-scoped diagnostics to the central Nevermind log. Entries are automatically tagged with the extension and command id.
- `ctx.logs.recent(options)` reads a bounded slice of the central Nevermind log for diagnostics. Options include `{ limit, level, source, sinceMs, query, extensionId }`; limits are capped by the host and results are structured entries, not raw filesystem access.
- `ctx.actions.toggleSetting(settingId, title)` and `ctx.actions.setPaletteShortcut(title)` for declarative settings actions
- `ctx.actions.recordShortcut({ actionId, scope, title })` opens the host-rendered shortcut recorder for an action. Use `scope: 'palette'` (or `actionId: '__palette-hotkey__'`) to record the global palette hotkey; otherwise pass the `actionId` of the target action. The host renders the recorder UI and persists the result.
- `ctx.actions.removeShortcut({ actionId, title })` removes a previously recorded shortcut by action id. Marked destructive by default.
- `ctx.ai.ask(prompt, options)` for a one-shot AI call that returns text. Options may include `{ system }`.
- `ctx.ai.session(id, options)` for a per-extension conversational AI session. The returned session supports `ask(prompt)` and `reset()`. Session ids are scoped to the extension, and options may include `{ system }`.
- `ctx.extension.rename(title)` or `ctx.extension.rename({ title, subtitle, commandTitle, commandSubtitle })` to persistently rename the extension metadata shown in search results
- `ctx.ui.item/actions/empty/loading/error` helpers
- `ctx.cache` is a per-extension TTL-aware in-memory cache for short-lived provider/view data. `cache.set(key, value, { ttlMs })` stores an entry with an optional time-to-live; `cache.get(key)` returns the value only while it is fresh, and `cache.getStale(key)` returns the last known value regardless of expiry so views can render an immediate stale snapshot while the next refresh runs. `cache.has(key)` reports freshness and `cache.invalidate(key?)` drops a single entry (or the whole namespace when called without arguments) and asks the host to refresh any tied root/search snapshots in the background. Entries are scoped to the extension id, live only for the running process, and intentionally do not persist — use `ctx.storage.memo/memoStale` for OS-level caching.
- `ctx.views.refresh()` returns a declarative action that re-runs the current command's `run(ctx)` and feeds the result back into the visible view as a `replace`-mode patch (or as a replacement view when the result has no items). Combine it with `refresh: { intervalMs, action: ctx.views.refresh(), mode: 'replace' }` to let the host poll while the view is open without overlapping refreshes or flicker. Host-applied refresh patches preserve the user's current selection and scroll position when the selected item still exists, and fall back to the nearest item only when it disappears; extensions should only send `selectedItemId` when they intentionally want to move focus. `ctx.views.invalidate()` drops the per-extension cache and the host-owned root snapshot for this extension so the next render rebuilds from scratch.
- `ctx.state` placeholder

`ctx.desktop.files.find(roots, options)` supports `{ limit, depth, extensions, kind, pattern, sortBy, order }`, where `kind` can be `image`, `video`, or `media`, and `sortBy` can be `recent`/`modified`, `added`, `created`, `name`, or `size`. `recent`/`modified` sorts by filesystem modification time (`mtimeMs`), which can be older than the download time when files preserve original metadata. `added` sorts by macOS Finder/Spotlight Date Added (`dateAddedMs`) with filesystem creation time as a fallback, and is usually the right choice for “newest files”, Downloads, screenshots, and mixed media galleries. `created` sorts by filesystem creation time (`birthtimeMs`). Convenience helpers `findImages`, `findVideos`, and `findMedia` call the same implementation. Returned files include `path`, `name`, `displayPath`, `url`, `fileUrl`, `videoUrl`, `thumbnailUrl`, `kind`, `extension`, `mtime`, `mtimeMs`, `birthtime`, `birthtimeMs`, `dateAdded`, `dateAddedMs`, and `size`. For grid videos, use `video: file.videoUrl` and `image: file.thumbnailUrl` to show a playable looping preview with a poster frame.

Use `await ctx.desktop.files.openWithApps(file.path)` to get installed apps that advertise support for that file type, then build an Open With nested view with `ctx.actions.openWith(file.path, app)`.

`ctx.storage` is scoped per extension file/identity, not per AI chat. `get`/`set`/`delete`/`clear` persist extension-owned state in app data, while `memo` and `memoStale` write only OS cache data so clearing the system/app cache removes them. `memo(key, ttlMs, loader)` caches expensive async work until the TTL expires. `memoStale(key, ttlMs, staleTtlMs, loader)` returns a stale cached value immediately while refreshing in the background, and only waits for `loader` when there is no usable cached value:

```js
const files = await ctx.storage.memoStale('recent-media', 60_000, 24 * 60 * 60_000, () =>
  ctx.desktop.files.findMedia(['~/Downloads', '~/Desktop'], { sortBy: 'added', limit: 200 })
)
```

## AI Builder (host-only)

The built-in `nevermind.ai-builder` extension is the only caller granted `ctx.aiBuilder.*`. The host gates this surface by extension id; public extensions get `undefined` for `ctx.aiBuilder` and a read-only `ctx.extensions.ownership` view. The AI Builder extension uses the same declarative primitives as every other extension — it is a reference implementation, not a privileged path through `executeAction`.

- `ctx.aiBuilder.startChat({ prompt, title? })` returns a declarative action that opens a fresh builder draft chat.
- `ctx.aiBuilder.openChat(chatId, { title? })` returns a declarative action that opens an existing chat.
- `ctx.aiBuilder.removeChat(chatId, { title? })` returns a destructive declarative action that removes only the chat history and AI session state. Generated extension files are durable and are never unlinked by chat removal; wrap with `ctx.ui.confirm(...)` to confirm before invoking.
- `ctx.aiBuilder.tweakExtension({ extensionFile, title?, prompt? })` returns a declarative action that opens (or creates) the tweak chat for a generated extension.
- `ctx.aiBuilder.openChatsList({ title? })` returns a declarative action that opens the chats list view.
- `ctx.aiBuilder.listChats()` and `ctx.aiBuilder.getChat(id)` are synchronous read helpers.

`ctx.extensions.ownership` exposes durable generated-extension ownership. Read methods (`ownerOf(extensionFile)`, `filesForChat(chatId)`, `canWrite(extensionFile, chatId)`) are available to every extension. Mutating methods (`claim(extensionFile, chatId)`, `reload()`) are exposed only to the AI Builder extension.

## AI builder write scope

The extension-building tools intentionally separate read access from write ownership. Builder chats can use `list_extensions` and `read_extension` to understand related generated extensions, but `write_extension` can only replace extension files already owned by the active chat. A single chat may own multiple related extension files. To change an extension owned by another chat, open that extension's tweak chat from the palette instead of overwriting it from the current conversation.

## Provider contracts

Root and search providers are the stable extension entry points the host calls to populate the palette. They follow these rules:

- **`rootItems(ctx)`** runs for empty-query renders. It must return an array of root items (or a `Promise` of one). The host calls it on palette open and after invalidations; results are cached per extension and reused across renders. Items contribute to host-owned ranking through `score` and the recency/usage signals attached by the host to each item id (boosts from past activations and `lastUsed`). Latency budget: results should complete inside the host timeout (currently ~10s); slower providers are dropped for that render. Return a small bounded list of high-signal items.
- **`searchItems(ctx, query)`** runs on each query change. The host debounces input before calling; providers should not introduce additional debounce. Return only matches relevant to `query`; the host applies a per-provider cap. Partial results are acceptable — return what is ready and rely on the next call for the rest.
- **Clone-safety**: returned items must be JSON-serializable. Functions, class instances, `Map`, `Set`, `Date`, and similar are not allowed. The only non-JSON shape the host accepts is the documented `__handler` marker on declarative actions produced by `ctx.actions.*`; do not invent other private fields.
- **Cache TTLs**: the host keeps a per-extension root snapshot for ~60s (`EXTENSION_ROOT_ITEMS_TTL_MS`). The `ctx.cache` namespace clamps `ttlMs` to a 24h ceiling and keeps at most 1000 entries per extension (oldest are evicted). Stale snapshots are served while the next refresh runs in the background; `ctx.cache.getStale(key)` returns the last value regardless of expiry.
- **Refresh triggers**: `ctx.cache.invalidate(key?)` drops a cache entry and asks the host to refresh tied root/search snapshots. `ctx.views.refresh()` re-runs the current command and patches the visible view in place. Refresh calls from a single extension are limited to ~5 per 2s window; bursts above the limit are dropped and logged.
- **Performance**: providers should target sub-100ms hot paths. Expensive work should use `ctx.storage.memo`/`memoStale` or `ctx.cache` so the visible list does not block on background refreshes. If a provider exceeds the host timeout the host drops that render's contribution and logs the failure — visible state from the prior render is preserved.

## Permissions

Extensions declare the host capabilities they need in a top-level `permissions` array on the manifest. The host gates the matching `ctx.*` surfaces: an undeclared capability becomes `undefined` on `ctx`, or throws `permission-denied` when called. Permissions are required for external extensions; internal extensions ship with explicit declarations and no implicit host privilege.

Available permissions:

| Permission | Grants |
| --- | --- |
| `desktop.apps` | `ctx.desktop.apps.*` |
| `desktop.files` | `ctx.desktop.files.*` |
| `clipboard.history` | `ctx.desktop.clipboard.*` |
| `ai` | `ctx.ai.ask`, `ctx.ai.session` |
| `extensions.ownership` | `ctx.extensions.ownership` (read-only; full access stays with the AI Builder extension) |
| `shortcuts` | `ctx.actions.recordShortcut`, `ctx.actions.removeShortcut`, `ctx.actions.setPaletteShortcut` |
| `system` | `ctx.desktop.shell.*` |
| `places` | declares use of host-owned filesystem places (no extra `ctx` surface today) |
| `updates` | declares participation in the update lifecycle (no extra `ctx` surface today) |
| `settings.write` | `ctx.settings.set`, `ctx.settings.toggle`, `ctx.actions.toggleSetting` |
| `camera` | declares use of host-owned camera views |

```ts
import type { NevermindExtension } from './nevermind-extension-api'

export default {
  id: 'my.images',
  title: 'My Images',
  permissions: ['desktop.files', 'clipboard.history'],
  // ...
} satisfies NevermindExtension
```

## Budgets

The host applies lightweight per-extension budgets so misbehaving providers cannot starve the palette:

- **Cache entries**: at most 1000 per extension; oldest entries are evicted when the limit is exceeded.
- **Cache TTL**: `ctx.cache.set` clamps `ttlMs` to a 24h maximum.
- **Refresh frequency**: `ctx.views.refresh()` is limited to 5 invocations per 2 seconds per extension. Excess calls are dropped (no view update, no error) and logged.
- **AI calls**: `ctx.ai.ask` and `ctx.ai.session().ask` are capped at 30 invocations per minute per extension. Over the limit, the call throws an `Error` with `code: 'ai-rate-limit-exceeded'`.

These limits are constants in `src/electron/main.ts` (`EXTENSION_CACHE_MAX_ENTRIES`, `EXTENSION_CACHE_MAX_TTL_MS`, `EXTENSION_REFRESH_MAX_BURST`, `EXTENSION_REFRESH_BURST_WINDOW_MS`, `EXTENSION_AI_CALLS_PER_MINUTE`) and are tuned for the host, not the extension; do not depend on specific values from inside an extension.

## Error handling

Extension command and action handlers may throw errors. Nevermind catches thrown errors and renders a native extension error view with the stack/message, so extensions should prefer throwing meaningful `Error` objects over swallowing failures or returning silent no-op states. Only catch errors when the extension can recover or add user-facing context before rethrowing.
