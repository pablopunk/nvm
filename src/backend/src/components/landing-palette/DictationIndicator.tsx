import { Check, Mic } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

const LISTENING_END_S = 2.2;
const PASTED_AT_S = 3.9;
const FADE_OUT_AT_S = 5.3;
const TOTAL_S = 5.6;
/** Sizes of src/native/internal/palette/aicommands/indicator.go. */
const RECORDING_WIDTH = 266;
const BASE_WIDTH = 218;
const WAVEFORM_SAMPLES = 32;
const WAVEFORM_HEIGHT = 20;
const MIN_LEVEL = 0.05;

type Phase = 'listening' | 'transcribing' | 'pasted';

function tween(time: number, from: number, to: number) {
  return Math.min(1, Math.max(0, (time - from) / (to - from)));
}

function phaseAt(time: number): Phase {
  if (time >= PASTED_AT_S) return 'pasted';
  if (time >= LISTENING_END_S) return 'transcribing';
  return 'listening';
}

/** Recent microphone levels around one baseline, newest on the right; silence stays a fine trace. */
function Waveform({ time }: { time: number }) {
  const phase = time * Math.PI * 2 * 0.8;
  return (
    <span className="wave" aria-hidden="true">
      {Array.from({ length: WAVEFORM_SAMPLES }, function renderSample(_, index) {
        const voice = Math.abs(Math.sin(phase - index * 0.4) * Math.sin(phase * 3 - index * 0.22));
        const level = MIN_LEVEL + (1 - MIN_LEVEL) * voice * (index / WAVEFORM_SAMPLES);
        return <span key={index} style={{ height: Math.max(1, level * WAVEFORM_HEIGHT) }} />;
      })}
    </span>
  );
}

/** The progress mark of StatusGlyph in src/native/internal/design/status_glyph.go. */
function ProgressGlyph() {
  return (
    <svg className="i spin" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
      <circle cx="10" cy="10" r="7" opacity="0.2" />
      <path d="M10 3a7 7 0 0 1 7 7" strokeLinecap="round" />
    </svg>
  );
}

const LABELS: Record<Phase, string> = {
  listening: 'Listening',
  transcribing: 'Transcribing',
  pasted: 'Pasted',
};

/**
 * The 2.x dictation indicator, replayed on a timer: it listens, transcribes, then pastes. It is the compact status
 * pill of src/native/internal/palette/aicommands/indicator.go: a toned mark, the state, and "Dictate" as context.
 */
export default function DictationIndicator({
  onPasted,
  onDone,
}: {
  onPasted: () => void;
  onDone: () => void;
}) {
  const [time, setTime] = useState(0);
  const callbacks = useRef({ onPasted, onDone });
  callbacks.current = { onPasted, onDone };

  useEffect(function runTimeline() {
    const started = performance.now();
    let frame = 0;
    let pasted = false;
    function tick(now: number) {
      const elapsed = (now - started) / 1000;
      if (elapsed >= TOTAL_S) {
        callbacks.current.onDone();
        return;
      }
      if (!pasted && elapsed >= PASTED_AT_S) {
        pasted = true;
        callbacks.current.onPasted();
      }
      setTime(elapsed);
      frame = requestAnimationFrame(tick);
    }
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, []);

  const phase = phaseAt(time);
  const visible = tween(time, 0, 0.25) * (1 - tween(time, FADE_OUT_AT_S, TOTAL_S));
  return (
    <div className="indicator" data-phase={phase} style={{ opacity: visible }} role="status">
      <div className="indicatorPane" style={{ width: phase === 'listening' ? RECORDING_WIDTH : BASE_WIDTH }}>
        <span className="mark">
          {phase === 'listening' ? <Mic className="i" aria-hidden="true" /> : null}
          {phase === 'transcribing' ? <ProgressGlyph /> : null}
          {phase === 'pasted' ? <Check className="i" aria-hidden="true" /> : null}
        </span>
        <strong key={phase}>{LABELS[phase]}</strong>
        <span className="divider" />
        <span className="context">Dictate</span>
        {phase === 'listening' ? <Waveform time={time} /> : null}
      </div>
    </div>
  );
}
