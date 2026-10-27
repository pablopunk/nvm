import baseCss from '../../../../app/palette/styles/base.css?inline';
import paletteCss from '../../../../app/palette/styles/palette.css?inline';
import themeCss from '../../../../app/palette/styles/theme.css?inline';
import viewsCss from '../../../../app/palette/styles/views.css?inline';

/**
 * The desktop app's own palette stylesheets, rebound for a shadow root: `:root`
 * becomes `:host`, and the dark theme always applies because the landing page
 * is dark regardless of the visitor's OS setting.
 */
const appCss = [themeCss, baseCss, paletteCss, viewsCss]
  .join('\n')
  .replaceAll(':root', ':host')
  .replaceAll('@media (prefers-color-scheme: dark)', '@media all');

const landingOverrides = `
:host { display: block; color: var(--text-primary); font-size: var(--fs-md); }
.palette { height: auto; }
.results { max-height: 392px; }
.card { -webkit-app-region: no-drag; }
`;

export const landingPaletteCss = `${appCss}\n${landingOverrides}`;
