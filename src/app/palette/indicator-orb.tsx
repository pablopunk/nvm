import { ThinkingOrb } from 'thinking-orbs';

type OrbState = 'listening' | 'working' | 'composing';

const ORB_STATE_BY_INDICATOR_STATUS: Record<string, OrbState> = {
  recording: 'listening',
  transcribing: 'composing',
  loading: 'working',
};

export function indicatorOrbState(status?: string) {
  return ORB_STATE_BY_INDICATOR_STATUS[String(status || '').toLowerCase()];
}

export function IndicatorOrb({ state }: { state: OrbState }) {
  return (
    <span className="indicatorOrb" aria-hidden="true">
      <ThinkingOrb state={state} size={20} />
    </span>
  );
}
