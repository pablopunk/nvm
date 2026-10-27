import { useEffect, useState } from 'react';
import { MODE_FRAMES, resolvePreset } from 'thinking-orbs/engine';

const LISTENING_END_S = 2.2;
const PASTED_AT_S = 3.9;
const FADE_OUT_AT_S = 5.3;
const TOTAL_S = 5.6;
const ORB_SIZE = 20;
const TRACE_WIDTH = 126;
const TRACE_SAMPLES = 48;
const MIN_LEVEL = 0.08;
const ORB_FLOOR = 0.35;
const PILL_BASE_WIDTH = 126;
const PILL_TRANSCRIBING_GROWTH = 34;
const PILL_PASTED_SHRINK = 56;

function tween(time: number, from: number, to: number) {
  return Math.min(1, Math.max(0, (time - from) / (to - from)));
}

function Orb({ state, time }: { state: 'listening' | 'composing'; time: number }) {
  const preset = resolvePreset(state, ORB_SIZE);
  const orb = MODE_FRAMES[preset.mode](ORB_SIZE, time * preset.speed, preset.opts);
  return (
    <svg width={ORB_SIZE} height={ORB_SIZE} viewBox={`0 0 ${ORB_SIZE} ${ORB_SIZE}`}>
      {orb.dots.map(function renderDot(dot, index) {
        const shade = Math.round(255 * (ORB_FLOOR + (1 - ORB_FLOOR) * dot.white));
        return (
          <circle
            key={index}
            cx={dot.x}
            cy={dot.y}
            r={dot.r}
            fill={`rgb(${shade},${shade},${shade})`}
            opacity={dot.a ?? 1}
          />
        );
      })}
    </svg>
  );
}

function MicTrace({ time, opacity }: { time: number; opacity: number }) {
  const phase = time * Math.PI * 2 * 0.8;
  return (
    <svg
      className="indicatorMicTrace"
      viewBox={`0 0 ${TRACE_WIDTH} 40`}
      preserveAspectRatio="none"
      aria-hidden="true"
      style={{ opacity }}
    >
      {Array.from({ length: TRACE_SAMPLES }, function renderSample(_, index) {
        const level =
          MIN_LEVEL +
          (1 - MIN_LEVEL) *
            Math.abs(
              Math.sin(phase - index * 0.4) * Math.sin(phase * 3 - index * 0.22),
            );
        const height = level * 36;
        return (
          <rect
            key={index}
            x={(index * TRACE_WIDTH) / (TRACE_SAMPLES - 1)}
            y={20 - height / 2}
            width={0.8}
            height={height}
            fill="var(--accent)"
          />
        );
      })}
    </svg>
  );
}

function SuccessMark({ progress }: { progress: number }) {
  return (
    <svg
      className="indicatorStatusIcon"
      data-kind="success"
      viewBox="0 0 20 20"
      width="20"
      height="20"
      fill="none"
      aria-hidden="true"
      style={{ ['--success-progress' as string]: progress }}
    >
      <circle className="indicatorStatusRing" cx="10" cy="10" r="8" />
      <path className="indicatorStatusGlyph" d="M6.5 10.5l2.5 2.5 4.5-5.5" />
    </svg>
  );
}

/**
 * The desktop app's dictation indicator, replayed on a timer: it listens,
 * transcribes, then pastes. Renders with the app's own indicator styles.
 */
export default function DictationIndicator({ onDone }: { onDone: () => void }) {
  const [time, setTime] = useState(0);

  useEffect(
    function runTimeline() {
      const started = performance.now();
      let frame = 0;
      function tick(now: number) {
        const elapsed = (now - started) / 1000;
        if (elapsed >= TOTAL_S) {
          onDone();
          return;
        }
        setTime(elapsed);
        frame = requestAnimationFrame(tick);
      }
      frame = requestAnimationFrame(tick);
      return () => cancelAnimationFrame(frame);
    },
    [onDone],
  );

  const transcribing = tween(time, LISTENING_END_S, LISTENING_END_S + 0.2);
  const pasted = tween(time, PASTED_AT_S, PASTED_AT_S + 0.2);
  const visible = tween(time, 0, 0.25) * (1 - tween(time, FADE_OUT_AT_S, TOTAL_S));
  const labels = [
    { text: 'Listening', opacity: 1 - transcribing },
    { text: 'Transcribing...', opacity: transcribing * (1 - pasted) },
    { text: 'Pasted', opacity: pasted },
  ];
  return (
    <div className="landingDictation extensionWindowChromeNone" style={{ opacity: visible }} role="status">
      <div
        className="extensionView progressView"
        style={{
          width: PILL_BASE_WIDTH + transcribing * PILL_TRANSCRIBING_GROWTH - pasted * PILL_PASTED_SHRINK,
        }}
      >
        <MicTrace time={time} opacity={(1 - transcribing) * 0.35} />
        <div
          className="progressOverview"
          data-status={pasted >= 1 ? 'done' : 'active'}
          data-mark="true"
        >
          <span className="dictationMark" aria-hidden="true">
            <span className="indicatorOrb" style={{ opacity: 1 - transcribing }}>
              <Orb state="listening" time={time} />
            </span>
            <span className="indicatorOrb" style={{ opacity: transcribing * (1 - pasted) }}>
              <Orb state="composing" time={time} />
            </span>
            <span style={{ opacity: pasted }}>
              <SuccessMark progress={pasted} />
            </span>
          </span>
          <div className="dictationLabel">
            {labels.map((label) => (
              <strong
                key={label.text}
                style={{ opacity: label.opacity, transform: `translateY(${(1 - label.opacity) * 4}px)` }}
              >
                {label.text}
              </strong>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
