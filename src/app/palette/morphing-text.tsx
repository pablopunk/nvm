import React from 'react';
import { TextMorph } from 'torph/react';

const MORPH_EASING = 'cubic-bezier(0.4, 0, 0.2, 1)';
const STREAMING_CHAT_MORPH_INTERVAL_MS = 100;
const STREAMING_CHAT_MORPH_MAX_CHARACTERS = 2_000;

export function MorphingIndicatorText({ value }: { value: string }) {
  return (
    <TextMorph
      className="indicatorMorphingText"
      duration={180}
      ease={MORPH_EASING}
      scale={false}
    >
      {value}
    </TextMorph>
  );
}

export function MorphingActivityText({ value }: { value: string }) {
  return (
    <span className="chatActivityGlow">
      <TextMorph
        className="chatActivityText"
        duration={220}
        ease={MORPH_EASING}
        scale={false}
        numbers={false}
      >
        {value}
      </TextMorph>
    </span>
  );
}

export function StreamingChatText({ value }: { value: string }) {
  const [visibleValue, setVisibleValue] = React.useState(value);
  const pendingValueRef = React.useRef(value);
  const flushTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );

  React.useEffect(() => {
    pendingValueRef.current = value;
    if (flushTimerRef.current) return;
    flushTimerRef.current = setTimeout(() => {
      flushTimerRef.current = null;
      setVisibleValue(pendingValueRef.current);
    }, STREAMING_CHAT_MORPH_INTERVAL_MS);
  }, [value]);

  React.useEffect(
    () => () => {
      if (flushTimerRef.current) clearTimeout(flushTimerRef.current);
    },
    [],
  );

  return (
    <TextMorph
      className="chatStreamingText"
      duration={160}
      ease={MORPH_EASING}
      scale={false}
      numbers={false}
      disabled={visibleValue.length > STREAMING_CHAT_MORPH_MAX_CHARACTERS}
    >
      {visibleValue}
    </TextMorph>
  );
}
