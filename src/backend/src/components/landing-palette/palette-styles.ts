import baseCss from '../../../../app/palette/styles/base.css?inline';
import extensionWindowsCss from '../../../../app/palette/styles/extension-windows.css?inline';
import paletteCss from '../../../../app/palette/styles/palette.css?inline';
import themeCss from '../../../../app/palette/styles/theme.css?inline';
import viewsCss from '../../../../app/palette/styles/views.css?inline';

/**
 * The desktop app's own palette stylesheets, rebound for a shadow root: `:root`
 * becomes `:host`, and the dark theme always applies because the landing page
 * is dark regardless of the visitor's OS setting.
 */
const appCss = [themeCss, baseCss, paletteCss, viewsCss, extensionWindowsCss]
  .join('\n')
  .replaceAll(':root', ':host')
  .replaceAll('@media (prefers-color-scheme: dark)', '@media all');

const landingOverrides = `
:host { display: block; color: var(--text-primary); font-size: var(--fs-md); }
.palette { height: auto; }
.results { max-height: 392px; }
.card { -webkit-app-region: no-drag; }
.landingDictation { position: absolute; left: 50%; top: min(calc(100% + 14px), 410px); width: 320px; transform: translateX(-50%); display: flex; justify-content: center; pointer-events: none; }
.landingDictation .progressView { flex: none; }
.landingDictation .progressOverview { width: 100%; }
.dictationMark { position: relative; flex: none; width: 20px; height: 20px; }
.dictationMark > span { position: absolute; inset: 0; }
.dictationLabel { position: relative; height: 18px; min-width: 0; flex: 1; }
.dictationLabel strong { position: absolute; top: 0; left: 0; }
.landingDictation .indicatorStatusGlyph { animation: none; stroke-dashoffset: calc(14 * (1 - var(--success-progress, 0))); }
.resultIcon.appIcon { background: transparent; box-shadow: none; }
.resultIcon.appIcon img { width: 32px; height: 32px; }
.resultText small.downloadNotice { color: var(--accent); }
.downloadNotice a { color: inherit; text-decoration: none; }
.tabHint a { color: var(--accent); text-decoration: none; }
.tabHint a:hover { text-decoration: underline; }
.downloadNotice a:hover { text-decoration: underline; }
`;

export const landingPaletteCss = `${appCss}\n${landingOverrides}`;
