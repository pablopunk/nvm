import React from 'react';
import { TextMorph } from 'torph/react';

const MORPH_EASING = 'cubic-bezier(0.4, 0, 0.2, 1)';
const STREAMING_CHAT_MORPH_INTERVAL_MS = 100;
const STREAMING_CHAT_MORPH_MAX_CHARACTERS = 2000;

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
  const activityTextRef = React.useRef<HTMLSpanElement>(null);

  React.useEffect(() => {
    const activityText = activityTextRef.current;
    if (!activityText) {
      return;
    }
    let animationFrame: number | null = null;

    function restartLetterAnimation() {
      if (animationFrame !== null) {
        cancelAnimationFrame(animationFrame);
      }
      const letters =
        activityText.querySelectorAll<HTMLElement>('[torph-item]');
      for (const letter of letters) {
        letter.classList.remove('chatActivityLetter');
      }
      animationFrame = requestAnimationFrame(() => {
        letters.forEach((letter, index) => {
          letter.style.setProperty('--activity-letter-index', String(index));
          letter.classList.add('chatActivityLetter');
        });
      });
    }

    restartLetterAnimation();
    const observer = new MutationObserver(restartLetterAnimation);
    observer.observe(activityText, { childList: true, subtree: true });
    return () => {
      observer.disconnect();
      if (animationFrame !== null) {
        cancelAnimationFrame(animationFrame);
      }
    };
  }, [value]);

  return (
    <span ref={activityTextRef} className="chatActivityGlow">
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
    if (flushTimerRef.current) {
      return;
    }
    flushTimerRef.current = setTimeout(() => {
      flushTimerRef.current = null;
      setVisibleValue(pendingValueRef.current);
    }, STREAMING_CHAT_MORPH_INTERVAL_MS);
  }, [value]);

  React.useEffect(
    () => () => {
      if (flushTimerRef.current) {
        clearTimeout(flushTimerRef.current);
      }
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
