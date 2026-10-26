import React, { useLayoutEffect, useRef } from 'react';
import { AbsoluteFill, useCurrentFrame } from 'remotion';
import { DesktopBackground, DesktopDock } from './desktop';
import { ProductUi } from './product-ui';
import {
  camera,
  PLAYBACK_SPEED,
  pointerAt,
  sceneAt,
  STORY,
  tween,
} from './timeline';

function Pointer({ frame }: { frame: number }) {
  const { x, y, clickAt, opacity } = pointerAt(frame);
  const click =
    tween(frame, clickAt - 3, clickAt) *
    (1 - tween(frame, clickAt, clickAt + 8));
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
          filter: `blur(${tween(frame, STORY.zoomStart, STORY.zoomPeak) * (1 - tween(frame, STORY.gridStart, STORY.gridSettled)) * 8}px)`,
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
