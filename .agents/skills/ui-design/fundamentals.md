# Fundamentals (Refactoring UI distilled)

The 80/20 of what makes interfaces look professional. Source: Refactoring UI by Adam Wathan & Steve Schoger.

## Hierarchy

Hierarchy is created by three levers, in order of impact:

1. **Size** — bigger draws the eye, but past a point, even bigger does nothing. Stop scaling and switch lever.
2. **Weight** — `font-weight: 600` vs `400` is often more effective than `+2px`.
3. **Color** — a medium gray label next to a near-black value reads cleaner than two same-color rows differentiated only by size.

Rules:

- One primary element per surface. Everything else is secondary or tertiary.
- De-emphasize secondary content deliberately. Hierarchy is not "primary loud"; it's "secondary quiet".
- Labels are usually less important than values. Style them as such (smaller, lighter, lower contrast).
- Don't use `font-weight < 400` for UI — it gets unreadable below 16px.

## Typography

- **Limit to two families.** One for UI, optionally one for display/marketing. Often one is enough.
- **Modular scale**: `12, 14, 16, 20, 24, 30, 36, 48`. Don't pick arbitrary sizes.
- **Line-height** scales inversely with size: ~1.5 for body, ~1.2 for headings, ~1.0 for very large display.
- **Measure (line length)**: aim for 45–75 characters per line for body text.
- **Numbers should be tabular** in tables and lists: `font-variant-numeric: tabular-nums`.
- **Letter-spacing**: tighten slightly on large headings, loosen slightly on ALL-CAPS labels.

## Spacing

- **Constrained scale**: `4, 8, 12, 16, 24, 32, 48, 64, 96` px.
- **Whitespace > borders.** Group related elements by proximity before drawing a box.
- **Asymmetric padding** is normal — a card may have `16px 24px`, not `20px 20px`.
- **Around vs between**: spacing between siblings is usually smaller than padding around the group.
- **Don't center vertically** large blocks of text; align to the top so reading position is stable.

## Color

- **5–9 shades per color.** Define `gray-50` through `gray-900`, and the same for any brand color.
- **Darkest is not black.** Use `#0a0a0a` or `#111827`; pure `#000` looks harsh and unnatural.
- **Lightest is not pure white** on dark mode; off-white reduces halation.
- **HSL beats hex for systematic palettes.** Vary L (and slightly S) along a fixed H to get a coherent ramp.
- **Saturation should drop at the extremes** — very light and very dark shades feel more natural with lower saturation.
- **Color != semantic.** Use semantic tokens: `text-primary`, `surface-1`, `border-subtle`, `accent`, `danger` — not `gray-700`.

### Dark mode

- Use dark grays for surfaces (`#0a0a0a`–`#1a1a1a`), not black.
- **Elevation by lightness**, not shadow. Higher surfaces are lighter, not raised by `box-shadow`.
- Off-white text (`#e5e7eb`), not pure white — reduces eye strain.
- **Re-check contrast** in dark mode independently. Inverting a light palette rarely passes.
- Reduce saturated color intensity for dark backgrounds — pure brand color often glows uncomfortably.

## Depth

- Soft, large, low-opacity shadows beat hard `0 2px 4px rgba(0,0,0,.5)`.
- Inset shadows for pressed states.
- Borders should be very low contrast — often a 1px line at 8–12% opacity is enough.
- Don't combine heavy border + shadow + background change — pick one.

## Forms

- Labels above inputs, not beside. Easier to scan, easier to localize.
- Required state is the default; mark **optional** fields, not required ones.
- Error messages: specific, near the field, with the field also in an error state.
- Disabled buttons must still be readable (AA contrast on label).
- Focus rings are mandatory and must be visible on every input and button.

## Icons

- Use one icon set across the whole app. Mixed styles look amateur.
- Icon size should match the cap-height of adjacent text, not the line-height.
- Icons are not decoration — every icon should aid comprehension or be removed.
