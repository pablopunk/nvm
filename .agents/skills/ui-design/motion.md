# Motion

Motion is feedback, not decoration. If an animation doesn't communicate state, cause, or hierarchy, remove it.

## When to animate

- **State change feedback** — a row becomes selected, a toggle flips, a button is pressed.
- **Spatial continuity** — a detail view slides in from the side so the user knows where they came from.
- **Attention** — a new toast appears, a value updates.
- **Affordance hint** — a button slightly scales on hover (subtle, ≤2%).

## When NOT to animate

- On every keystroke.
- On list-item entry when filtering (just swap, instantly).
- On color changes that are pure theme application.
- Anything that competes with the user's current task.

## Durations

| Type | Duration |
| --- | --- |
| Micro-interaction (hover, press, focus) | 100–150ms |
| Small transition (toggle, tooltip) | 150–200ms |
| Panel slide / sheet | 250–350ms |
| Cross-screen transition | 300–500ms |
| Long-form (rare, like onboarding reveal) | 500–800ms |

Past 500ms, animations feel slow no matter how good the easing.

## Easing

- **Ease-out** for enter (decelerate into rest): `cubic-bezier(0.16, 1, 0.3, 1)` — Material's emphasized-decelerate.
- **Ease-in** for exit (accelerate away): `cubic-bezier(0.4, 0, 1, 1)`.
- **Ease-in-out** for in-place changes (toggle, color swap): `cubic-bezier(0.4, 0, 0.2, 1)`.
- Avoid `linear` (mechanical) and `ease` (mushy default).
- **Spring physics** for playful interactions, but only with stiffness ≥ 300 and damping ≥ 25 — sloppy springs feel cheap.

## What to animate

Cheap, GPU-accelerated:

- `transform: translate / scale / rotate`
- `opacity`
- `filter: blur` (sparingly)

Avoid animating:

- `width`, `height`, `top`, `left`, `margin` — layout thrash.
- `box-shadow` — repaint cost.
- `background-color` of large surfaces — flicker on Electron.

If you must resize, animate `transform: scale` and adjust the layout silently after.

## Accessibility

Always respect `prefers-reduced-motion`:

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

For meaningful transitions (orientation, spatial), provide a non-motion equivalent (instant swap) rather than just removing them.

## Loading & progress

- 0–100ms: nothing. The user doesn't notice.
- 100–400ms: micro-feedback only (button stays pressed, input shows a faint pulse).
- 400ms–2s: indeterminate spinner or skeleton.
- >2s: progress bar with stages if possible. Never a spinner alone for long waits.

Skeletons should match the final layout's shape and animate with a subtle shimmer (1.5s loop, ease-in-out).

## Palette-specific

- Open: 150–200ms slide+fade from -8px Y, ease-out.
- Close: 100–120ms fade+slide to -4px Y, ease-in.
- Result rows: **no transition** on filter. Instant swap.
- Selection move: **no animation**. The selected background is a CSS background-color change with no transition.
- Detail view slide-in: 250ms, translate X from 100% with overshoot ≤2px.
