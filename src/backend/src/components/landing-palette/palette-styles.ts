import videoPaletteCss from '../../../marketing/demo-video/assets/palette.css?inline';

/**
 * The 2.x dark token set (`darkTokens` in src/native/internal/design/tokens.go), plus the pane material the demo
 * video uses. The landing page is dark whatever the visitor's OS setting, so only the dark set applies.
 */
const darkTokens = `
:host {
  --pane: rgba(26, 31, 40, 0.86); --pane-edge: rgba(211, 224, 245, 0.14);
  --bg: rgba(26, 31, 40, 0.94); --surface: #222834; --raised: #222834; --interactive: #29303d; --selected: #343d4d;
  --edge-subtle: rgba(211, 224, 245, 0.1); --edge-strong: rgba(211, 224, 245, 0.18);
  --strong: #f0f3f7; --primary: #e0e5ec; --secondary: #b8c0cc; --tertiary: #9ca6b4; --muted: #8f99a7;
  --faint: rgba(143, 153, 167, 0.75); --link: #d7c574;
  --accent: #f1cf54; --accent-fill: #e2bd39; --accent-contrast: #242015;
  --accent-soft: rgba(241, 207, 84, 0.12); --accent-muted: rgba(241, 207, 84, 0.24); --accent-emph: rgba(241, 207, 84, 0.5);
  --success: #9fc9a8; --success-soft: rgba(159, 201, 168, 0.12);
  --hi: rgba(220, 231, 249, 0.1); --lo: rgba(0, 0, 0, 0.5); --shadow: rgba(0, 0, 0, 0.55);
  --tone-yellow: #dfce84; --tone-blue: #a9c9df; --tone-purple: #c8b6df; --tone-green: #acd1b2;
  --tone-red: #e0aaaa; --tone-orange: #dfbd96;
}
`;

/** The page around the shared parts: the pane at its 2.x size, real inputs, and the scroll areas. */
const pageCss = `
:host {
  display: block; color: var(--strong); font-size: 13px; line-height: normal; text-align: left;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
}
*, *::before, *::after { box-sizing: border-box; }
p { margin: 0; }
button { margin: 0; padding: 0; border: 0; background: none; color: inherit; font: inherit; cursor: pointer; text-align: inherit; }
input, textarea { margin: 0; border: 0; background: none; color: var(--strong); font: inherit; outline: none; }
input::placeholder, textarea::placeholder { color: var(--faint); opacity: 1; }
a { color: var(--link); text-decoration: none; }
a:hover { text-decoration: underline; }

.stage { position: relative; }
.pane {
  position: relative; height: 470px; overflow: hidden; border-radius: 18px; color: var(--strong);
  background: var(--pane); backdrop-filter: blur(40px) saturate(1.6); -webkit-backdrop-filter: blur(40px) saturate(1.6);
  box-shadow: 0 0 0 0.5px var(--pane-edge), 0 24px 64px rgba(0, 0, 0, 0.42);
}

.searchPage { display: flex; flex-direction: column; height: 100%; }
.search { flex: none; }
.field input { flex: 1; min-width: 0; height: 26px; padding: 0; font-size: 21px; caret-color: var(--strong); }
button.chip { flex: none; line-height: normal; }
button.chip:hover { color: var(--strong); }

.list, .menuList { overflow-y: auto; scrollbar-width: none; overscroll-behavior: contain; }
.list::-webkit-scrollbar, .menuList::-webkit-scrollbar { display: none; }
.list { flex: 1; min-height: 0; padding-bottom: 52px; scroll-padding: 11px 0 52px; }
.row { cursor: pointer; user-select: none; }
.tx .t, .tx .s { display: block; }
/* The bundled app icons are flat squares; macOS draws app icons as rounded squares. */
.tile.app img { border-radius: 9px; }
.s.notice a { color: var(--accent); }

.grow { flex: 1; }
.footer { pointer-events: none; }
.footer > * { pointer-events: auto; }
.hint, .plate.name { min-width: 0; }
.hint .ht, .plate.name .ht { overflow: hidden; text-overflow: ellipsis; }
.hint .keycap { flex: none; }
.plate.name { max-width: 50%; }
.plate.hints { min-width: 0; max-width: 100%; }

.menu {
  position: absolute; right: 12px; bottom: 44px; z-index: 5; width: 290px; max-width: calc(100% - 24px); padding: 5px;
  border-radius: 12px; background: var(--raised); border: 1px solid var(--edge-strong);
  box-shadow: inset 0 1px 0 var(--hi), 0 14px 34px var(--shadow), 0 2px 6px var(--lo);
}
.menuSearch { display: flex; align-items: center; gap: 8px; padding: 8px 10px; }
.menuSearch .i { width: 13px; height: 13px; flex: none; color: var(--muted); }
.menuSearch input { flex: 1; min-width: 0; font-size: 13px; }
.menuSearch input::placeholder { color: var(--muted); }
.etched { height: 4px; }
.menuList { max-height: 260px; }
.menuRow {
  display: flex; align-items: center; gap: 8px; padding: 6px 10px; border-radius: 8px; border: 1px solid transparent;
  font-size: 13px; color: var(--strong); cursor: pointer; user-select: none;
}
.menuRow[data-selected="true"] {
  background: var(--selected); border-color: var(--edge-subtle); box-shadow: inset 0 1px 0 var(--hi), 0 1px 3px var(--lo);
}
.menuRow .mt { flex: 1; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.menuRow .mk { font-size: 11px; font-weight: 500; color: var(--muted); }
.menuRow .i { width: 12px; height: 12px; flex: none; color: var(--muted); }
.menuEmpty { padding: 10px; font-size: 12px; color: var(--muted); }

.chatPage { position: absolute; inset: 0; }
.chat { bottom: 141px; overflow-y: auto; scrollbar-width: none; }
.chat::-webkit-scrollbar { display: none; }
.answer { margin-top: 12px; }
.composer {
  display: block; width: calc(100% - 32px); resize: none; line-height: 18px; color: var(--strong);
}

.indicator {
  position: absolute; left: 50%; bottom: 60px; z-index: 10; transform: translateX(-50%); pointer-events: none;
}
.indicatorPane {
  display: flex; align-items: center; gap: 10px; height: 38px; padding: 6px 10px; overflow: hidden; border-radius: 20px;
  background: var(--bg); border: 1px solid var(--edge-subtle);
  box-shadow: inset 0 1px 0 var(--hi), 0 1px 3px var(--lo), 0 12px 32px rgba(0, 0, 0, 0.45);
  transition: width 0.2s ease;
}
.indicatorPane .mark {
  flex: none; display: flex; align-items: center; justify-content: center; width: 24px; height: 24px;
  border-radius: 12px; color: var(--accent); background: var(--accent-soft);
}
.indicator[data-phase="pasted"] .mark { color: var(--success); background: var(--success-soft); }
.indicatorPane .mark .i { width: 15px; height: 15px; }
.indicator[data-phase="listening"] .mark .i { animation: indicatorPulse 1.2s ease-in-out infinite; }
.indicatorPane .spin { color: inherit; animation: indicatorSpin 0.9s linear infinite; }
.indicatorPane strong {
  flex: 1; min-width: 0; font-size: 13px; font-weight: 500; color: var(--strong); white-space: nowrap;
  animation: indicatorLabel 0.2s ease;
}
.indicatorPane .divider { flex: none; width: 1px; height: 12px; background: var(--edge-strong); }
.indicatorPane .context { flex: none; font-size: 11px; color: var(--muted); }
.indicatorPane .wave { flex: none; display: flex; align-items: center; gap: 1px; height: 20px; }
.indicatorPane .wave span { width: 1px; border-radius: 1px; background: var(--accent); }
@keyframes indicatorSpin { to { transform: rotate(360deg); } }
@keyframes indicatorPulse { 50% { opacity: 0.78; } }
@keyframes indicatorLabel { from { opacity: 0; transform: translateY(4px); } }
@media (prefers-reduced-motion: reduce) {
  .indicatorPane .spin, .indicatorPane .mark .i, .indicatorPane strong { animation: none; }
}
@media (max-width: 520px) {
  .plate.name { display: none; }
}
`;

/**
 * The 2.x palette for a shadow root. Rows, tiles, the search row, chips, the footer and the chat parts come from the
 * landing demo video's stylesheet, so the page and the video share one copy of the Go components' look.
 */
export const landingPaletteCss = `${darkTokens}\n${videoPaletteCss}\n${pageCss}`;
