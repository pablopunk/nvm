# UI Review Checklist

Run this on every visual change before considering it done. Yes/no answers only — if "no" or "unsure", fix or justify.

## Hierarchy

- [ ] Is there exactly one primary action on this surface?
- [ ] Is the most important element the most visually prominent (size, weight, color — in that order)?
- [ ] Is secondary content deliberately de-emphasized (lighter color, smaller, lower weight)?
- [ ] Could you understand the screen in grayscale?

## Typography

- [ ] All font sizes come from the modular scale (12/14/16/20/24/30/36/48)?
- [ ] No font weight below 400 in UI?
- [ ] Line height ~1.5 for body, ~1.2 for headings?
- [ ] Numbers in tables use `font-variant-numeric: tabular-nums`?
- [ ] System font stack only — no webfonts for chrome?

## Spacing

- [ ] All padding/margin values come from the spacing scale (4/8/12/16/24/32/48/64)?
- [ ] Generous whitespace (start too much, then remove)?
- [ ] Related items grouped by proximity, not boxes?
- [ ] Around > between (group padding > sibling gap)?

## Color

- [ ] Darkest "black" is not pure `#000`?
- [ ] Lightest "white" is not pure `#fff` (especially in light mode UI chrome)?
- [ ] Contrast ratio ≥ 4.5:1 for normal text, ≥ 3:1 for large text and UI components?
- [ ] Re-checked contrast in dark mode independently?
- [ ] Color is used semantically (token names like `text-primary`, not `gray-700`)?

## Native feel (desktop)

- [ ] Cursor on buttons is `default`, not `pointer`?
- [ ] Text selection disabled on UI chrome (buttons, labels, menus)?
- [ ] Uses system font stack?
- [ ] Drag region set on title bar, no-drag on its interactive children?
- [ ] Window appears only after `ready-to-show` (no white flash)?
- [ ] Respects system dark/light mode?
- [ ] Shortcuts use `CommandOrControl` and render correctly per OS?

## Keyboard

- [ ] Every action reachable by mouse is reachable by keyboard?
- [ ] Tab order is logical?
- [ ] Visible focus ring on every interactive element?
- [ ] `Escape` closes overlays / modals / palette?
- [ ] Shortcuts displayed inline next to actions where applicable?

## Motion

- [ ] Every animation communicates state, cause, or hierarchy (not decoration)?
- [ ] Durations: micro 100–200ms, transitions 200–400ms, none >500ms?
- [ ] Only `transform` and `opacity` (not `width`/`height`/`box-shadow`)?
- [ ] Easing: ease-out enter, ease-in exit, ease-in-out in-place?
- [ ] Respects `prefers-reduced-motion`?

## Forms

- [ ] Labels above inputs?
- [ ] Optional fields marked, not required ones?
- [ ] Error messages specific and near the field?
- [ ] Disabled state still readable (AA contrast)?

## States

- [ ] Empty state designed (not blank)?
- [ ] Loading state designed (skeleton or contained spinner)?
- [ ] Error state designed (inline, recoverable)?
- [ ] No-results state has a fallback action?

## Command palette (when applicable)

- [ ] Single column results?
- [ ] Row shows: icon · label · secondary label · shortcut?
- [ ] Selected row uses subtle background tint, not border?
- [ ] Mouse hover and keyboard selection share the same selected style?
- [ ] Shortcuts shown inline?
- [ ] Result groups have light section headers (small, dim, uppercase)?
- [ ] Fuzzy match with prefix > word-start > acronym > anywhere ranking?
- [ ] No animation on filter / selection move?

## Accessibility

- [ ] WCAG AA contrast in both themes?
- [ ] Color is never the only signal (icons / labels / patterns also convey state)?
- [ ] Focus indicators visible?
- [ ] Keyboard fully functional without mouse?
- [ ] `aria-*` attributes on custom widgets?

## Final smell test

- [ ] Would this screen look at home next to Raycast / Linear / Arc?
- [ ] Is there anything you'd be embarrassed to demo?
- [ ] Have you tried it for 60 seconds as a real user, not just inspected it?

If all boxes check, ship.
