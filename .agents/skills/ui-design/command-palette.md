# Command Palette UX

Patterns specific to keyboard-first command palette interfaces — Nevermind's core surface. Reference apps: Raycast, Linear, Arc, VS Code, Spotlight, GitHub `?`-menu.

## Invocation

- **One global shortcut**, memorable, single-handed if possible. `⌘K` is the convention; `⌘Space` competes with Spotlight.
- The palette must appear instantly (<100ms perceived) — preload, don't lazy-init.
- Re-pressing the shortcut while open should toggle close, not re-open.
- **Escape** closes. Always. Never trap focus.

## Window & framing

- Floating panel above existing windows; not full-screen.
- Center horizontally, top-third vertically — feels lighter than dead-center.
- Width ~640–720px on desktop; capped, not responsive to monitor width.
- Subtle backdrop blur or scrim if there's content behind; never fully opaque overlay.
- Single bordered container, no toolbar chrome inside the palette itself.

## Input

- Search input is the only top-level affordance. No tabs, no buttons next to it.
- **Placeholder** explains what's possible: "Search apps, files, or type a command…"
- Auto-focus on open. Always.
- **Fuzzy match**, not exact substring. Score by: prefix > word-start > acronym > anywhere.
- Recency and frequency boost rank.
- Show the matched characters bolded or highlighted in results.
- Clear (`⌘⌫` or `Esc` once) resets to empty state, doesn't close.

## Results list

- **Single column.** Multi-column palettes are slower to scan.
- Row height ~40–48px. Tight enough to fit ~8 results without scroll, loose enough to be clickable.
- Each row: icon (16–20px) · primary label · secondary label (right-aligned, dim) · shortcut (far right, monospace).
- **Selected row** has a subtle filled background — not a border, not a glow.
- Mouse hover and keyboard selection share the same selected style; moving the mouse should update keyboard selection.
- Result groups: light section headers (`SUGGESTIONS`, `COMMANDS`, `EXTENSIONS`) — uppercase, small, dim, no dividers needed.
- Cap visible results (8–12) and scroll; never show 100 results unfiltered.

## Keyboard model

| Action | Shortcut |
| --- | --- |
| Open / close | `⌘K` |
| Move selection | `↑` `↓` |
| Jump to first/last | `⌘↑` `⌘↓` |
| Run action | `↵` |
| Run secondary | `⌘↵` or `⇧↵` |
| Open action menu | `⌘.` |
| Tab into detail | `→` |
| Back / clear | `Esc` or `⌘⌫` |
| Quick-run favorited | `⌘1` … `⌘9` |

- Every action visible in the palette must declare its shortcut, even if redundant.
- Never use `Tab` to move selection — `Tab` is reserved for focus traversal inside detail panes.

## Completeness

- The palette should include **every** action available via the menu bar, every context menu, and every keyboard shortcut.
- If a feature isn't in the palette, users won't find it. Treat the palette as the canonical action surface; the menu bar mirrors it.

## States

- **Empty (initial)** — show suggestions, recent items, or pinned commands. Never a blank list.
- **Empty (no results)** — short message + "Create a new …" or "Search the web for …" fallback. Never just "No results".
- **Loading** — show skeleton rows or a tiny inline spinner in the input chevron. Don't replace the whole result area with a spinner.
- **Error** — inline at the top of results, not as a modal.

## Actions & sub-views

- Primary action is `↵`. Secondary actions live in an inline action menu (`⌘.`), not as buttons.
- For detail views (preview, form, multi-step), slide-in from the right of the same panel. Keep the search input visible if it still applies.
- Breadcrumbs at the top-left for nested views; Escape pops one level.

## AI-specific considerations (Nevermind)

- Streaming output is part of the surface. Use a calm, monospaced area; don't reflow the whole palette on each token.
- AI suggestions should be visually distinct from local results (subtle accent color or icon), and ranked, not prepended unconditionally.
- "Ask AI" should be a stable fallback row at the bottom of empty-results states.
- Extensions installed by AI should appear in the same list as built-ins — no second-class section.

## Anti-patterns

- Tabs at the top of the palette. The input is the filter; tabs duplicate it.
- Multi-select with checkboxes. The palette is for single-action invocation.
- Modal-on-modal. If you need to confirm, slide a detail view in, don't stack.
- Hiding shortcuts behind hover. Always visible.
- Animating on every keystroke. The list should update instantly with no transition.
