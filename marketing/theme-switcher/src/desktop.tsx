import React from 'react';
import { Img, staticFile } from 'remotion';
import { camera, STORY, themeTransition } from './timeline';
import THEMES from './themes.json';

const ICONS = [
  'finder',
  'calendar',
  'notes',
  'photos',
  'calculator',
  'settings',
  'nevermind',
];

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

export function DesktopDock({ frame }: { frame: number }) {
  const { theme, previous, changedAt } = themeTransition(frame);
  const iconDelay =
    changedAt >= STORY.dockZoomStart ? 24 / (ICONS.length - 1) : 0.5;
  return (
    <div className="desktopDock">
      {ICONS.map(function renderIcon(icon, index) {
        const iconTheme =
          frame >= changedAt + index * iconDelay ? theme : previous;
        return (
          <React.Fragment key={icon}>
            <div className="dockIcon">
              <Img
                src={staticFile(`dock-${icon}-${THEMES[iconTheme].id}.png`)}
              />
              {[0, 2, 6].includes(index) ? (
                <span className="dockRunningDot" />
              ) : null}
            </div>
          </React.Fragment>
        );
      })}
    </div>
  );
}
