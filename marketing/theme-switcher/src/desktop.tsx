import React from 'react';
import { Img, interpolate, staticFile } from 'remotion';
import { camera, themeAt, tween } from './timeline';

const THEMES = [
  { wallpaper: null, hue: 0 },
  { wallpaper: 'pink', hue: 105 },
  { wallpaper: 'forest', hue: -55 },
  { wallpaper: 'meadow', hue: 0 },
];

const ICONS = [
  'finder',
  'calculator',
  'calendar',
  'ardia',
  'photos',
  'slack',
  't3',
  'twitter',
  'network',
  'settings',
  'zoom',
  'whatsapp',
  'chrome',
];

function themeTransition(frame: number) {
  const theme = themeAt(frame);
  const previous = frame < 449 ? 0 : frame < 539 ? 1 : 2;
  const changedAt =
    theme === 1 ? 357 : theme === 2 ? 449 : theme === 3 ? 539 : -100;
  return { theme, previous, mix: tween(frame, changedAt, changedAt + 14) };
}

export function DesktopBackground({ frame }: { frame: number }) {
  const { theme, previous, mix } = themeTransition(frame);
  const view = camera(frame);
  const halfWidth = 960 / view.scale;
  const halfHeight = 540 / view.scale;
  const x = Math.max(halfWidth, Math.min(1920 - halfWidth, view.x));
  const y = Math.max(halfHeight, Math.min(1080 - halfHeight, view.y));
  return (
    <div
      className="desktopBackground"
      style={{
        transform: `translate(960px, 540px) scale(${view.scale}) translate(${-x}px, ${-y}px)`,
      }}
    >
      <div className="desktopBlank" />
      {THEMES[previous].wallpaper ? (
        <Img
          className="desktopWallpaper"
          src={staticFile(`${THEMES[previous].wallpaper}.jpg`)}
        />
      ) : null}
      {THEMES[theme].wallpaper ? (
        <Img
          className="desktopWallpaper"
          src={staticFile(`${THEMES[theme].wallpaper}.jpg`)}
          style={{ opacity: mix }}
        />
      ) : null}
      <div className="desktopShade" />
    </div>
  );
}

function iconFilter(frame: number, index: number) {
  const { theme, previous, mix } = themeTransition(frame);
  let hue = interpolate(mix, [0, 1], [THEMES[previous].hue, THEMES[theme].hue]);
  let saturation = theme === 0 ? 0 : previous === 0 ? mix : 1;
  const waves = [
    { start: 666, hue: 105 },
    { start: 707, hue: -55 },
    { start: 748, hue: 0 },
  ];
  for (const wave of waves) {
    const progress = tween(
      frame,
      wave.start + index * 2,
      wave.start + index * 2 + 14,
    );
    hue = interpolate(progress, [0, 1], [hue, wave.hue]);
    saturation = Math.max(saturation, progress);
  }
  return `saturate(${saturation}) hue-rotate(${hue}deg)`;
}

export function DesktopDock({ frame }: { frame: number }) {
  return (
    <div className="desktopDock">
      {ICONS.map(function renderIcon(icon, index) {
        return (
          <React.Fragment key={icon}>
            {index === 10 ? <span className="dockDivider" /> : null}
            <div className="dockIcon">
              <Img
                src={staticFile(`dock-${icon}.png`)}
                style={{ filter: iconFilter(frame, index) }}
              />
              {[0, 3, 6, 10, 11, 12].includes(index) ? (
                <span className="dockRunningDot" />
              ) : null}
            </div>
          </React.Fragment>
        );
      })}
    </div>
  );
}
