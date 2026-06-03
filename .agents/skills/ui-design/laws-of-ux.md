# Laws of UX (applied)

Heuristics worth internalizing. Each has a concrete application to Nevermind's surfaces.

## Fitts's Law

Time to acquire a target is a function of its size and distance.

- Make primary actions larger and place them near the user's current focus (the input, the selected row).
- Screen edges and corners are "infinite-size" targets — use them for global shortcuts and the menu bar.
- Don't shrink a destructive button to make it less prominent; instead, color and position it away.

## Hick's Law

Decision time grows with the number of options.

- The palette mitigates Hick by filtering on keystroke — the user is never looking at 200 options, only the top 8.
- Group commands and use section headers so users scan groups, not items.
- In settings, hide advanced options behind a disclosure. Show 3 toggles, not 30.

## Jakob's Law

Users prefer interfaces that work like the ones they already know.

- Steal patterns from Raycast/Linear/VS Code rather than inventing. `⌘K` is universal — don't bind it to anything else.
- Match macOS HIG for menus, sheets, and shortcuts.
- Don't be clever with iconography. A gear is settings, a magnifier is search.

## Miller's Law

Short-term memory holds ~7 ±2 items.

- 8–12 visible results in the palette is the sweet spot.
- Section headers chunk lists into recognizable groups, raising effective capacity.
- Multi-step flows: ≤5 steps with progress indicator, or it feels like a form swamp.

## Tesler's Law (conservation of complexity)

Every system has irreducible complexity — someone bears it. Push it to the app, not the user.

- AI extension builder hides the complexity of writing extension code. The user describes intent; we handle the rest.
- Smart defaults > settings. Every preference is a tax.

## Aesthetic-Usability Effect

Users perceive better-designed interfaces as more usable, and forgive their flaws.

- Polish pays. Spending an hour on type rhythm and spacing changes the perceived quality of the whole app.

## Doherty Threshold

Productivity soars when system response is under 400ms.

- Palette open <100ms perceived. Search results <50ms after keystroke. Action execution <200ms or show progress.
- If something must take longer, show progress immediately — never leave a 400ms+ blank.

## Goal-Gradient Effect

Motivation increases as you approach a goal.

- Onboarding: show progress (3 of 5). Even if the steps are identical, perceived effort drops.

## Peak-End Rule

People remember the peak and the end of an experience, not the average.

- Make the success state delightful (subtle animation, clear confirmation). It's the "end" of every interaction.
- Errors should recover gracefully — a good error recovery is remembered better than a smooth path.

## Postel's Law (robustness)

Be liberal in what you accept, conservative in what you produce.

- Palette search: accept typos, partial matches, different cases, aliases.
- Render: deterministic, consistent, no jitter on every keystroke.

## Law of Proximity

Things close together are perceived as related.

- Group label + value with tight spacing; separate groups with larger spacing. Borders rarely needed.

## Law of Common Region

Items in a bounded region are perceived as a group.

- Use subtle background tint instead of borders to group rows or cards.

## Law of Similarity

Visually similar items are perceived as related.

- All destructive actions in one color. All AI-generated content with one marker. Don't mix metaphors.
