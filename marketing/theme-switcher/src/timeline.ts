import { Easing, interpolate } from 'remotion';

export const REQUEST = 'I want to create themes for my mac';
export const FPS = 30;
export const PLAYBACK_SPEED = 1.15;
export const DURATION = Math.ceil(810 / PLAYBACK_SPEED);

export const THEME_CHANGES = [
  { frame: 0, theme: 0 },
  { frame: 357, theme: 1 },
  { frame: 449, theme: 2 },
  { frame: 539, theme: 3 },
  { frame: 666, theme: 1 },
  { frame: 707, theme: 2 },
  { frame: 748, theme: 3 },
];

export const BEATS = [
  { frame: 0, name: 'Palette', scale: 1, x: 960, y: 540 },
  { frame: 30, name: 'Type request', scale: 1.72, x: 920, y: 320 },
  { frame: 118, name: 'Create with AI', scale: 1.72, x: 920, y: 320 },
  { frame: 145, name: 'Click create', scale: 1.9, x: 900, y: 355 },
  { frame: 174, name: 'Slide in chat', scale: 1.25, x: 960, y: 450 },
  { frame: 250, name: 'Building', scale: 1.25, x: 960, y: 450 },
  { frame: 274, name: 'Zoom transition', scale: 5, x: 850, y: 380 },
  { frame: 285, name: 'Wallpaper grid', scale: 1, x: 960, y: 540 },
  { frame: 345, name: 'Choose pink', scale: 1, x: 960, y: 540 },
  { frame: 370, name: 'Pink desktop', scale: 1, x: 960, y: 540 },
  { frame: 414, name: 'Choose forest', scale: 1, x: 960, y: 540 },
  { frame: 454, name: 'Forest desktop', scale: 1, x: 960, y: 540 },
  { frame: 504, name: 'Choose meadow', scale: 1, x: 960, y: 540 },
  { frame: 544, name: 'Meadow desktop', scale: 1, x: 960, y: 540 },
  { frame: 610, name: 'Zoom to Dock', scale: 1, x: 960, y: 540 },
  { frame: 660, name: 'Dock color sweep', scale: 2.25, x: 960, y: 1008 },
  { frame: 809, name: 'Hold', scale: 2.25, x: 960, y: 1008 },
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
  if (frame < 174) return 'palette';
  if (frame < 279) return 'chat';
  if (frame < 600) return 'grid';
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
    mix: index === 0 ? 1 : tween(frame, change.frame, change.frame + 10),
  };
}
