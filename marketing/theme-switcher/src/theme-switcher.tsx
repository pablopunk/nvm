import React, { useLayoutEffect, useRef } from 'react';
import { AbsoluteFill, interpolate, useCurrentFrame } from 'remotion';
import { DesktopBackground, DesktopDock } from './desktop';
import { ProductUi } from './product-ui';
import { camera, sceneAt, tween } from './timeline';

function Pointer({ frame }: { frame: number }) {
  const create = frame < 174;
  const start = frame >= 504 ? 504 : frame >= 414 ? 414 : 304;
  const progress = create
    ? tween(frame, 119, 153)
    : tween(frame, start, start + 27);
  const target = create
    ? { x: 1060, y: 280 }
    : frame >= 504
      ? { x: 1240, y: 550 }
      : frame >= 414
        ? { x: 1240, y: 330 }
        : { x: 960, y: 330 };
  const x = interpolate(progress, [0, 1], [1460, target.x]);
  const y = interpolate(progress, [0, 1], [740, target.y]);
  const clickAt = create ? 158 : frame >= 504 ? 530 : frame >= 414 ? 440 : 348;
  const click =
    tween(frame, clickAt - 3, clickAt) *
    (1 - tween(frame, clickAt, clickAt + 8));
  const opacity = create
    ? tween(frame, 117, 123)
    : tween(frame, start, start + 7);
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
  const frame = useCurrentFrame();
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
