import { Easing, interpolate } from 'remotion';
import { existingExtensionAt } from './extension-scenes';

export const REQUEST = 'mac theme switcher';
export const FPS = 30;
export const PLAYBACK_SPEED = 1.15;
export const INTRO_FRAMES = 24;
export const CONTENT_FRAMES = Math.ceil(960 / PLAYBACK_SPEED);
export const OUTRO_FRAMES = 48;
export const DURATION = INTRO_FRAMES + CONTENT_FRAMES + OUTRO_FRAMES;

export const STORY = {
  typingStart: 24,
  typingInterval: 1.7,
  createMoveStart: 67,
  createMoveEnd: 74,
  createClick: 77,
  chatStart: 92,
  chatSettled: 104,
  zoomStart: 158,
  zoomPeak: 179,
  gridStart: 184,
  gridSettled: 191,
  dismissStart: 260,
  dismissEnd: 274,
  dockZoomStart: 248,
  dockZoomEnd: 276,
};

export const GRID_CLICKS = [
  { frame: 210, theme: 2, x: 1240, y: 330 },
  { frame: 234, theme: 1, x: 960, y: 330 },
  { frame: 258, theme: 3, x: 1240, y: 550 },
];

export const THEME_CHANGES = [
  { frame: 0, theme: 0 },
  ...GRID_CLICKS,
  { frame: 301, theme: 2 },
  { frame: 336, theme: 1 },
  { frame: 371, theme: 3 },
];

export const BEATS = [
  { frame: 0, scale: 1, x: 960, y: 540 },
  { frame: 18, scale: 1.72, x: 920, y: 320 },
  { frame: 64, scale: 1.72, x: 920, y: 320 },
  { frame: STORY.createClick, scale: 1.72, x: 920, y: 320 },
  { frame: STORY.chatSettled, scale: 1.25, x: 960, y: 450 },
  { frame: STORY.zoomStart, scale: 1.25, x: 960, y: 450 },
  { frame: STORY.zoomPeak, scale: 5, x: 850, y: 380 },
  { frame: STORY.gridSettled, scale: 1, x: 960, y: 540 },
  { frame: STORY.dockZoomStart, scale: 1, x: 960, y: 540 },
  { frame: STORY.dockZoomEnd, scale: 2.25, x: 960, y: 1008 },
  { frame: 425, scale: 2.25, x: 960, y: 1008 },
  { frame: 448, scale: 1.55, x: 960, y: 360 },
  { frame: 480, scale: 1.55, x: 960, y: 360 },
  { frame: 504, scale: 1.38, x: 960, y: 450 },
  { frame: 550, scale: 1.38, x: 960, y: 450 },
  { frame: 560, scale: 1.55, x: 960, y: 360 },
  { frame: 603, scale: 1.55, x: 960, y: 360 },
  { frame: 619, scale: 1.38, x: 960, y: 450 },
  { frame: 675, scale: 1.38, x: 960, y: 450 },
  { frame: 704, scale: 1, x: 960, y: 540 },
  { frame: 724, scale: 1, x: 960, y: 540 },
  { frame: 770, scale: 1.12, x: 960, y: 500 },
  { frame: 959, scale: 1.12, x: 960, y: 500 },
];

const easing = Easing.bezier(0.65, 0, 0.2, 1);

export function tween(frame: number, from: number, to: number) {
  return interpolate(frame, [from, to], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
    easing,
  });
}

export function camera(frame: number) {
  const points = BEATS.map((beat) => beat.frame);
  const options = {
    extrapolateLeft: 'clamp' as const,
    extrapolateRight: 'clamp' as const,
    easing,
  };
  return {
    scale: interpolate(
      frame,
      points,
      BEATS.map((beat) => beat.scale),
      options,
    ),
    x: interpolate(
      frame,
      points,
      BEATS.map((beat) => beat.x),
      options,
    ),
    y: interpolate(
      frame,
      points,
      BEATS.map((beat) => beat.y),
      options,
    ),
  };
}

export function sceneAt(frame: number) {
  if (existingExtensionAt(frame)) return 'existing-extension';
  if (frame < STORY.chatStart) return 'palette';
  if (frame < STORY.gridStart) return 'chat';
  if (frame < STORY.dismissEnd) return 'grid';
  return 'desktop';
}

export function themeTransition(frame: number) {
  let index = THEME_CHANGES.length - 1;
  while (index > 0 && THEME_CHANGES[index].frame > frame) index -= 1;
  const change = THEME_CHANGES[index];
  const previous = THEME_CHANGES[Math.max(0, index - 1)].theme;
  return {
    theme: change.theme,
    previous,
    changedAt: change.frame,
    mix: index === 0 ? 1 : tween(frame, change.frame, change.frame + 6),
  };
}

export function pointerAt(frame: number) {
  const extension = existingExtensionAt(frame);
  if (extension) {
    const progress = tween(frame, extension.moveStart, extension.click - 3);
    return {
      x: interpolate(progress, [0, 1], [1460, 1060]),
      y: interpolate(
        progress,
        [0, 1],
        [740, extension.id === 'pipeline' ? 510 : 280],
      ),
      opacity:
        tween(frame, extension.moveStart - 2, extension.moveStart) *
        (1 - tween(frame, extension.click + 3, extension.listStart)),
      clickAt: extension.click,
    };
  }
  if (frame < STORY.chatStart) {
    const progress = tween(frame, STORY.createMoveStart, STORY.createMoveEnd);
    return {
      x: interpolate(progress, [0, 1], [1460, 1060]),
      y: interpolate(progress, [0, 1], [740, 280]),
      opacity: tween(frame, STORY.createMoveStart - 2, STORY.createMoveStart),
      clickAt: STORY.createClick,
    };
  }
  let x = 1460;
  let y = 740;
  let clickAt = GRID_CLICKS[0].frame;
  for (const click of GRID_CLICKS) {
    if (frame >= click.frame - 10) clickAt = click.frame;
    const progress = tween(frame, click.frame - 10, click.frame - 3);
    x = interpolate(progress, [0, 1], [x, click.x]);
    y = interpolate(progress, [0, 1], [y, click.y]);
  }
  return {
    x,
    y,
    opacity: tween(frame, 198, 200) * (1 - tween(frame, 266, 270)),
    clickAt,
  };
}
