import { Easing, interpolate } from 'remotion';

export const REQUEST = 'I want to create themes for my mac';
export const FPS = 30;
export const DURATION = 810;

export const BEATS = [
  { frame: 0, name: 'Palette', scale: 1, x: 960, y: 540 },
  { frame: 30, name: 'Type request', scale: 1.72, x: 920, y: 320 },
  { frame: 118, name: 'Create with AI', scale: 1.72, x: 920, y: 320 },
  { frame: 145, name: 'Click create', scale: 1.9, x: 900, y: 355 },
  { frame: 174, name: 'Slide in chat', scale: 1.25, x: 960, y: 450 },
  { frame: 250, name: 'Building', scale: 1.25, x: 960, y: 450 },
  { frame: 274, name: 'Zoom transition', scale: 5, x: 850, y: 380 },
  { frame: 285, name: 'Wallpaper grid', scale: 1.15, x: 960, y: 510 },
  { frame: 345, name: 'Choose pink', scale: 1.15, x: 960, y: 510 },
  { frame: 370, name: 'Pink desktop', scale: 1, x: 960, y: 540 },
  { frame: 414, name: 'Choose forest', scale: 1.05, x: 960, y: 540 },
  { frame: 454, name: 'Forest desktop', scale: 1, x: 960, y: 540 },
  { frame: 504, name: 'Choose meadow', scale: 1.05, x: 960, y: 540 },
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
  if (
    frame < 357 ||
    (frame >= 414 && frame < 449) ||
    (frame >= 504 && frame < 539)
  )
    return 'grid';
  return 'desktop';
}

export function themeAt(frame: number) {
  if (frame < 357) return 0;
  if (frame < 449) return 1;
  if (frame < 539) return 2;
  return 3;
}
