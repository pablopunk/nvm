import type React from 'react';
import { TextMorph } from 'torph/react';

const MORPH_EASING = 'cubic-bezier(0.4, 0, 0.2, 1)';
const activityGraphemeSegmenter = new Intl.Segmenter(undefined, {
  granularity: 'grapheme',
});

function activityLetterEntries(value: string) {
  return Array.from(activityGraphemeSegmenter.segment(value), (entry) => ({
    id: `${entry.index}:${entry.segment}`,
    grapheme: entry.segment === ' ' ? '\u00a0' : entry.segment,
  }));
}

export function activityGraphemes(value: string) {
  return activityLetterEntries(value).map(({ grapheme }) => grapheme);
}

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
        duration={320}
        ease={MORPH_EASING}
        scale={false}
        numbers={false}
      >
        {value}
      </TextMorph>
      <span key={value} className="chatActivityLetters" aria-hidden="true">
        {activityLetterEntries(value).map(({ grapheme, id }, index) => (
          <span
            key={id}
            style={
              {
                '--activity-letter-index': index,
              } as React.CSSProperties
            }
          >
            {grapheme}
          </span>
        ))}
      </span>
    </span>
  );
}
