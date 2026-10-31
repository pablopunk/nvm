# Compact Action Panel Placement

## Problem

On the packaged Linux build, pressing Ctrl+K in an empty My Tasks list showed only a thin edge of the action menu. Enter still ran Add Task, so the menu worked but could not be seen.

## Root cause

The panel hangs from a zero-height anchor at the end of the pane flow. The shared panel rule sizes it with a percentage `max-height`, and a percentage of a zero-height containing block resolves to zero, so the panel collapsed to its border. The old smoke check only compared the panel bottom with the card bottom, which a collapsed panel satisfies, and the CSS-string unit test could not see the collapse at all.

## Fix

The panel keeps hanging from the last card edge. Its `max-height` is now measured: `compactPanelMaxHeight` limits it to the room between the palette top and that edge, with a floor that keeps the filter row and one action usable. The panel is a flex column so its action list scrolls instead of being clipped.

## Verification

`compact-panel-placement.test.ts` covers the arithmetic. The Electron smoke specs assert real rendered bounds: panel visible, inside the window, hanging above the card edge, with a visible action row, and a scrollable list for a menu longer than a short card.

## Limits

Stacked sibling views are covered by the placement arithmetic only, not by an end-to-end placement assertion. Packaged-build behavior on Linux, macOS, and Windows still needs a packaged run.
