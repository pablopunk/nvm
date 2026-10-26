import React, { useLayoutEffect, useRef } from 'react';
import { AbsoluteFill, interpolate, useCurrentFrame } from 'remotion';
import { DesktopBackground, DesktopDock } from './desktop';
import { ProductUi } from './product-ui';
import { camera, PLAYBACK_SPEED, sceneAt, tween } from './timeline';

function Pointer({ frame }: { frame: number }) {
  const create = frame < 174;
  const progress = tween(frame, 119, 153);
  const positions = [279, 304, 338, 402, 433, 492, 523, 600];
  const options = {
    extrapolateLeft: 'clamp' as const,
    extrapolateRight: 'clamp' as const,
  };
  const x = create
    ? interpolate(progress, [0, 1], [1460, 1060])
    : interpolate(
        frame,
        positions,
        [1460, 1460, 960, 960, 1240, 1240, 1240, 1240],
        options,
      );
  const y = create
    ? interpolate(progress, [0, 1], [740, 280])
    : interpolate(
        frame,
        positions,
        [740, 740, 330, 330, 330, 330, 550, 550],
        options,
      );
  const clickAt = create ? 158 : frame >= 492 ? 530 : frame >= 402 ? 440 : 348;
  const click =
    tween(frame, clickAt - 3, clickAt) *
    (1 - tween(frame, clickAt, clickAt + 8));
  const opacity = create
    ? tween(frame, 117, 123)
    : tween(frame, 304, 311) * (1 - tween(frame, 546, 560));
  return (
    <div
      className="videoPointer"
      style={{
        left: x,
        top: y,
        opacity,
        transform: `scale(${1 - click * 0.18})`,
      }}
    >
      <svg width="30" height="38" viewBox="0 0 30 38">
        <path
          d="M3 2L3 29L10 23L16 35L22 32L16 21L26 21Z"
          fill="white"
          stroke="#12151b"
          strokeWidth="2"
          strokeLinejoin="round"
        />
      </svg>
    </div>
  );
}

export function ThemeSwitcher() {
  const frame = useCurrentFrame() * PLAYBACK_SPEED;
  const scene = sceneAt(frame);
  const view = camera(frame);
  const stage = useRef<HTMLDivElement>(null);
  useLayoutEffect(
    function seekProductAnimations() {
      for (const animation of stage.current?.getAnimations({ subtree: true }) ||
        []) {
        animation.pause();
        animation.currentTime = (frame / 30) * 1000;
      }
    },
    [frame, scene],
  );
  return (
    <AbsoluteFill className="videoCanvas">
      <DesktopBackground frame={frame} />
      <div
        ref={stage}
        className="videoStage"
        style={{
          transform: `translate(960px, 540px) scale(${view.scale}) translate(${-view.x}px, ${-view.y}px)`,
          filter: `blur(${tween(frame, 254, 274) * (1 - tween(frame, 279, 287)) * 8}px)`,
        }}
      >
        <DesktopDock frame={frame} />
        {scene !== 'desktop' ? <ProductUi frame={frame} scene={scene} /> : null}
        {scene === 'palette' || scene === 'grid' ? (
          <Pointer frame={frame} />
        ) : null}
      </div>
    </AbsoluteFill>
  );
}
