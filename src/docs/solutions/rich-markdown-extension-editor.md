# Rich Markdown Extension Editor

## Problem or symptoms

An extension editor declared with `format: 'markdown'` still behaved like a
plain textarea. Markdown markers remained visible, while enabling the existing
preview produced a separate split pane that duplicated the note and made a
small floating window feel crowded.

The same surface also exposed two related interaction problems:

- A separate rename prompt duplicated information already present in the first
  line of a note.
- Cmd+K actions replaced the whole independent window instead of staying
  secondary to the document.

## Context

Floating Notes dogfoods `ctx.ui.editor`, host-owned draft autosave, persistent
extension windows, and action panels. The editor must retain Markdown as its
portable storage format while rendering formatting in place and preserving
Nevermind's palette keyboard model.

## What did not work

- A textarea cannot style individual Markdown ranges, so CSS alone cannot turn
  it into an in-place rich editor.
- A split preview renders Markdown but is not the single-surface editing model
  expected in a compact notes window.
- Exporting the entire rich document to Markdown synchronously on every input
  event puts document-sized work directly on the typing path.
- Letting Cmd+K bubble from a rich editor is unreliable because editor plugins
  may consume the shortcut first.

## Root cause

The original editor model treated Markdown as plain text plus an optional
preview. Rich editing requires a document model that can import Markdown,
render structured nodes, accept Markdown shortcuts, and export the same content
back to Markdown. Independent-window actions also needed a presentation mode
that did not reuse the full replacement surface.

## Fix

- `src/markdown-editor.tsx` uses Lexical with the standard Markdown
  transformers and registered heading, quote, list, link, and code nodes.
- The editor imports and exports Markdown, so extension storage and draft
  autosave remain format-compatible rather than persisting Lexical state or
  HTML.
- Markdown export is coalesced before updating React and draft state, keeping
  full-document serialization off the raw keystroke path. Pending editor state
  flushes directly to the host draft on unmount so an immediate close cannot
  lose the final edit through an ignored parent state update.
- `src/editor-title.ts` derives a plain title from the first non-empty content
  line, stripping common block and inline Markdown markers. Note-like editors
  opt into the shared `titleFromContent` view contract instead of adding rename
  UI.
- The extension-window Command root handles local shortcuts in the capture
  phase so Cmd+K remains host-owned even when the rich editor has focus.
- `actionPanelPresentation: 'compact'` keeps the editor visible and renders the
  same filterable action and confirmation rows in a small bottom-right overlay.

## Verification

Run:

```bash
mise exec -- pnpm typecheck
mise exec -- pnpm test
mise exec -- pnpm build
```

Dogfood Floating Notes and verify:

- Markdown headings, emphasis, links, code, lists, and checklists render in
  place while remaining editable.
- The first non-empty line becomes the saved/searchable title without a rename
  action.
- Cmd+K opens compact actions while the note remains visible.
- Filtering, Enter, destructive confirmation, Escape, autosave, and reopening
  preserve their behavior.

## Notes for future searches

Keywords: Floating Notes, ctx.ui.editor, Markdown, Lexical,
MarkdownShortcutPlugin, TRANSFORMERS, titleFromContent, draft autosave,
contenteditable, Cmd+K capture, actionPanelPresentation, compact action panel,
extension window.
