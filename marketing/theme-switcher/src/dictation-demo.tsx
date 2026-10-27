import React from 'react';
import { AbsoluteFill, useCurrentFrame } from 'remotion';
import { MODE_FRAMES, resolvePreset } from 'thinking-orbs/engine';
import { IndicatorStatusIcon } from '../../../src/app/palette/indicator/indicator-status-icon';
import { tween } from './timeline';

const TRANSCRIBING = 60;
const PASTED = 102;
export const DICTATION_DURATION = 150;

function Orb({
  state,
  time,
}: {
  state: 'listening' | 'composing';
  time: number;
}) {
  const preset = resolvePreset(state, 20);
  const orb = MODE_FRAMES[preset.mode](20, time * preset.speed, preset.opts);
  return (
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
  );
}

export function DictationDemo() {
  const frame = useCurrentFrame();
  const phase = (frame / 120) * Math.PI * 2;
  const transcribing = tween(frame, TRANSCRIBING, TRANSCRIBING + 6);
  const pasted = tween(frame, PASTED, PASTED + 6);
  const states = [
    { label: 'Listening', opacity: 1 - transcribing },
    { label: 'Transcribing...', opacity: transcribing * (1 - pasted) },
    { label: 'Pasted', opacity: pasted },
  ];
  return (
    <AbsoluteFill className="videoCanvas dictationDemo">
      <div
        className="extensionWindowChromeNone dictationDemoIndicator"
        style={{ opacity: tween(frame, 0, 8) * (1 - tween(frame, 141, 149)) }}
      >
        <div
          className="extensionView progressView"
          style={
            {
              width: 126 + transcribing * 34 - pasted * 56,
              '--success-progress': pasted,
            } as React.CSSProperties
          }
        >
          <svg
            className="indicatorMicTrace"
            viewBox="0 0 126 40"
            preserveAspectRatio="none"
            aria-hidden="true"
            style={{ opacity: (1 - transcribing) * 0.35 }}
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
            data-status={frame >= PASTED ? 'done' : 'active'}
            data-mark="true"
          >
            <span className="dictationDemoMark" aria-hidden="true">
              <span
                className="indicatorOrb"
                style={{ opacity: 1 - transcribing }}
              >
                <Orb state="listening" time={frame / 30} />
              </span>
              <span
                className="indicatorOrb"
                style={{ opacity: transcribing * (1 - pasted) }}
              >
                <Orb state="composing" time={frame / 30} />
              </span>
              <span style={{ opacity: pasted }}>
                <IndicatorStatusIcon kind="success" />
              </span>
            </span>
            <div className="dictationDemoLabel">
              {states.map(function renderLabel(state) {
                return (
                  <strong
                    key={state.label}
                    style={{
                      opacity: state.opacity,
                      transform: `translateY(${(1 - state.opacity) * 4}px)`,
                    }}
                  >
                    {state.label}
                  </strong>
                );
              })}
            </div>
          </div>
        </div>
      </div>
    </AbsoluteFill>
  );
}
