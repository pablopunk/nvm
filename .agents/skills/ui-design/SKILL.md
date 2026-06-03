---
name: ui-design
description: Use when designing, building, or reviewing UI in Nevermind — the command palette, extension UIs, settings, onboarding, or any visual surface. Covers visual fundamentals (Refactoring UI), command-palette UX patterns, desktop/Electron/macOS conventions, motion, dark mode, and accessibility. Trigger on requests like "design X", "make this look better", "review the UI", "polish this screen", or any change to a `.tsx`/`.css` file under `src/renderer` or `src/ui`.
---

# UI Design

Nevermind is an AI-native desktop command palette built on Electron + Vite + React. Its UI must feel keyboard-first, fast, native on macOS, and polished at the level of Raycast, Linear, and Arc. This skill is the curated knowledge base for designing and reviewing that UI.

## How to use this skill

When the task is to design or review UI:

1. Start with `checklist.md` — run it as a review pass on the change.
2. Pull deeper guidance from supporting docs on demand:
   - `fundamentals.md` — typography, spacing, color, hierarchy, dark mode (Refactoring UI distilled).
   - `command-palette.md` — keyboard-first palette UX, search, results, shortcuts, empty/loading states.
   - `desktop-electron.md` — macOS HIG + Electron conventions to avoid the uncanny valley.
   - `laws-of-ux.md` — heuristics (Fitts, Hick, Jakob, Miller, Tesler, etc.) with concrete applications.
   - `motion.md` — durations, easing, when to animate, when not to.
3. Never invent design tokens. If Nevermind already defines spacing, color, or type scales, reuse them. If it doesn't, propose adding them once and use them everywhere.

## Core rules (always apply)

- **Keyboard-first.** Every action reachable by mouse must be reachable by keyboard, and the keyboard path must be at least as fast. Show shortcuts inline next to actions.
- **One primary action per surface.** Demote everything else with weight, size, or color — not by hiding it.
- **Design in grayscale first.** Hierarchy comes from size, weight, and contrast; color is the last layer, not the first.
- **Constrained scales.** Use a fixed spacing scale (4/8/12/16/24/32/48/64). Type scale is **compact**: `11/12/13/14/16/18` — not the generic 12/14/16/20/24 ramp. Nevermind is a dense palette, not a marketing page.
- **Concentric radii.** Inner radius = outer radius − padding. A card at 24px with 8px inner padding → child elements at 16px. Never nest radii that fight each other.
- **Compact over airy.** Tight, information-dense layouts beat marketing-grade whitespace. Match Raycast/Linear density, not Stripe landing pages.
- **Native cursors and selection.** Default cursor on buttons (not `pointer`). Disable text selection on UI chrome. `pointer` only for true links.
- **System fonts.** `-apple-system, BlinkMacSystemFont, "SF Pro", ui-sans-serif, system-ui` — never ship a webfont for chrome.
- **No focus ring on text inputs.** The search input, chat textarea, and form fields are always-focused or single-focus surfaces — the caret is the focus indicator. Keep `:focus-visible` rings on **buttons and links only**.
- **Motion is feedback, not decoration.** 100–200ms for micro-interactions, 200–400ms for transitions. Ease-out for entering, ease-in for leaving. Respect `prefers-reduced-motion`.
- **WCAG AA minimum.** 4.5:1 contrast for normal text, 3:1 for large text and UI components — in both light and dark mode.
- **No pure black, no pure white.** Use `#0a0a0a`–`#111827` for darkest, off-white for lightest. Pure `#000` on `#fff` causes halation.

## Non-goals

- This skill is not a brand guide. It does not pick logos, illustration style, or marketing voice.
- It is not for marketing pages or onboarding microcopy (other than UI labels).

## When in doubt

Compare against Raycast, Linear, and Arc — those are the bar. If a screen would feel out of place next to them, it's not done.
