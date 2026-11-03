import React, { useLayoutEffect, useRef } from 'react';
import { AbsoluteFill, staticFile, useCurrentFrame } from 'remotion';
import { DesktopBackground, DesktopDock } from './desktop';
import { ExistingExtensions } from './existing-extensions';
import { ProductUi } from './product-ui';
import {
  camera,
  CONTENT_FRAMES,
  INTRO_FRAMES,
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
  const outputFrame = useCurrentFrame();
  const frame =
    Math.max(0, Math.min(CONTENT_FRAMES - 1, outputFrame - INTRO_FRAMES)) *
    PLAYBACK_SPEED;
  const intro = outputFrame < INTRO_FRAMES;
  const openingSwipe = tween(outputFrame, 6, INTRO_FRAMES - 1);
  const outroFrame = outputFrame - INTRO_FRAMES - CONTENT_FRAMES;
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
          transform: `translateY(${(1 - openingSwipe) * 1200}px) translate(960px, 540px) scale(${view.scale}) translate(${-view.x}px, ${-view.y}px)`,
          filter: `blur(${tween(frame, STORY.zoomStart, STORY.zoomPeak) * (1 - tween(frame, STORY.gridStart, STORY.gridSettled)) * 8}px)`,
        }}
      >
        <DesktopDock frame={frame} />
        {scene === 'existing-extension' ? (
          <ExistingExtensions frame={frame} />
        ) : scene !== 'desktop' ? (
          <ProductUi frame={frame} scene={scene} />
        ) : null}
        {scene === 'palette' ||
        scene === 'grid' ||
        scene === 'existing-extension' ? (
          <Pointer frame={frame} />
        ) : null}
      </div>
      {intro || outroFrame >= 0 ? (
        <AbsoluteFill style={{ pointerEvents: 'none' }}>
          <AbsoluteFill
            style={{
              background: 'black',
              opacity: intro ? 0 : tween(outroFrame, 0, 24),
            }}
          />
          <AbsoluteFill
            style={{
              alignItems: 'center',
              justifyContent: 'center',
              opacity: intro ? 1 : tween(outroFrame, 14, 32),
              transform: intro
                ? `translateY(${-openingSwipe * 720}px)`
                : undefined,
            }}
          >
            <img
              src={staticFile('wordmark.png')}
              alt="Nevermind"
              style={{ width: 800, height: 'auto' }}
            />
          </AbsoluteFill>
        </AbsoluteFill>
      ) : null}
    </AbsoluteFill>
  );
}
