# Reference screenshots (Electron palette)

Screenshots of the Electron app's palette, kept as the visual reference for the Go rewrite. They are for
human review side by side with Go renders, not for pixel diffing across toolkits: our own goldens under
`testdata/golden/` are the regression net. Files are copied unchanged; do not edit them.

Regeneration from the Electron app is described in `src/native/scripts/reference-capture.md`.

| File | Shows | Appearance | Date (of the file) | How produced |
|---|---|---|---|---|
| `palette-dotted-connectors-light.png` | Palette with the dotted pane connectors (900 x 820) | light | added 2026-10-14 in `58134a43` (`feat(palette): add dotted pane connectors`) | copied from `docs/screenshots/`; produced by the Electron app for that commit, capture method not recorded in the repo |
| `palette-dotted-connectors-dark.png` | Same view as above (900 x 820) | dark | added 2026-10-14 in `58134a43` | copied from `docs/screenshots/`, as above |
| `root-search-automate.png` | Root search with the `Automate "..."` fallback row and the web search row (1280 x 800) | light | 2026-11-03 | captured from the Electron app on the Linux box during the Electron investigation (session scratchpad, file `p1.png`); capture script not recorded |
| `clipboard-history-preview.png` | Clipboard history view with a preview pane (1280 x 800) | light | 2026-11-03 | captured from the Electron app on the Linux box (session scratchpad, file `m3.png`); capture script not recorded |
| `ai-chat.png` | AI chat view (1280 x 800) | light | 2026-11-03 | captured from the Electron app on the Linux box (session scratchpad, file `ai1.png`); capture script not recorded |
| `root-automate-tab-hint.png` | Root view with the `Automate` fallback and the `Tab` hint (1280 x 800) | light | 2026-11-03 | captured from the Electron app on the Linux box (session scratchpad, file `d1.png`); capture script not recorded |

Dark references exist only for the dotted-connectors view. The four Linux-box captures are light only. In
`root-search-automate.png` the palette pane is an opaque paper fill with no wallpaper showing through it, so
these captures do not show the translucent palette; the open problem is in `reference-capture.md`.
