const MAXIMUM_FRAME_GAP_MS = 250;
const REQUIRED_SIGNAL_FRAME_COUNT = 3;
const SILENT_FALLBACK_MS = 5_000;
const BLUETOOTH_SAMPLE_RATE_MAX = 32_000;

export function needsBluetoothMicrophoneReadiness(
  label: string,
  sampleRate: number | undefined,
) {
  return (
    (typeof sampleRate === 'number' &&
      sampleRate <= BLUETOOTH_SAMPLE_RATE_MAX) ||
    /airp(?:od|ol)|bluetooth|beats|bose|jabra|headset/i.test(label)
  );
}

export function createMicrophoneReadinessTracker(
  needsBluetoothReadiness: boolean,
) {
  let streakStartedAt: number | undefined;
  let lastFrameAt: number | undefined;
  let framesSinceSignal = 0;

  return {
    consume(now: number, hasSignal: boolean) {
      if (!needsBluetoothReadiness) return true;
      if (
        lastFrameAt === undefined ||
        now - lastFrameAt > MAXIMUM_FRAME_GAP_MS
      ) {
        streakStartedAt = now;
        framesSinceSignal = 0;
      } else if (framesSinceSignal > 0) {
        framesSinceSignal += 1;
      }
      if (hasSignal && framesSinceSignal === 0) framesSinceSignal = 1;
      lastFrameAt = now;
      return (
        framesSinceSignal >= REQUIRED_SIGNAL_FRAME_COUNT ||
        now - (streakStartedAt ?? now) >= SILENT_FALLBACK_MS
      );
    },
  };
}
