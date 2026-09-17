export const MIC_LEVEL_SMOOTHING_DECAY = 0.6;

export function clampMicLevel(value: number) {
  if (!Number.isFinite(value)) return 0;
  return Math.min(1, Math.max(0, value));
}

export function micLevelDisplayValue(peak: number) {
  return Math.sqrt(clampMicLevel(peak));
}

export function smoothMicLevel(previous: number, peak: number) {
  return Math.max(
    micLevelDisplayValue(peak),
    previous * MIC_LEVEL_SMOOTHING_DECAY,
  );
}
