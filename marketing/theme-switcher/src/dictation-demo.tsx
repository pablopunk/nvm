import React from 'react';
import { AbsoluteFill, Img, staticFile, useCurrentFrame } from 'remotion';
import { MODE_FRAMES, resolvePreset } from 'thinking-orbs/engine';

const DURATION = 120;
const preset = resolvePreset('listening', 20);

export function DictationDemo() {
  const frame = useCurrentFrame();
  const phase = (frame / DURATION) * Math.PI * 2;
  const time = (1 - Math.cos(phase)) * 1.5;
  const orb = MODE_FRAMES[preset.mode](20, time * preset.speed, preset.opts);
  return (
    <AbsoluteFill className="videoCanvas dictationDemo">
      <Img className="dictationDemoWallpaper" src={staticFile('meadow.jpg')} />
      <div className="extensionWindowChromeNone dictationDemoIndicator">
        <div className="extensionView progressView">
          <svg
            className="indicatorMicTrace"
            viewBox="0 0 126 40"
            preserveAspectRatio="none"
            aria-hidden="true"
          >
            {Array.from({ length: 48 }, function renderSample(_, index) {
              const level =
                0.08 +
                0.92 *
                  Math.abs(
                    Math.sin(phase - index * 0.4) *
                      Math.sin(phase * 3 - index * 0.22),
                  );
              const height = level * 36;
              return (
                <rect
                  key={index}
                  x={(index * 126) / 47}
                  y={20 - height / 2}
                  width={0.8}
                  height={height}
                  fill="var(--accent)"
                />
              );
            })}
          </svg>
          <div
            className="progressOverview"
            data-status="active"
            data-mark="true"
          >
            <span className="indicatorOrb" aria-hidden="true">
              <svg width="20" height="20" viewBox="0 0 20 20">
                {orb.dots.map(function renderDot(dot, index) {
                  const ink = Math.round((1 - dot.white) * 255);
                  return (
                    <circle
                      key={index}
                      cx={dot.x}
                      cy={dot.y}
                      r={dot.r}
                      fill={`rgb(${ink},${ink},${ink})`}
                      opacity={dot.a ?? 1}
                    />
                  );
                })}
              </svg>
            </span>
            <div>
              <strong>Listening</strong>
            </div>
          </div>
        </div>
      </div>
    </AbsoluteFill>
  );
}
