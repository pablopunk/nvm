import {
  createMicrophoneReadinessTracker,
  needsBluetoothMicrophoneReadiness,
} from './dictation-readiness';
import { smoothMicLevel } from './mic-level';

const MICROPHONE_READY_TIMEOUT_MS = 7_000;
const MICROPHONE_SIGNAL_THRESHOLD = 0.000_01;
const MAX_DICTATION_AUDIO_BYTES = 4_194_304;

export async function recordDictation(
  deviceId: string | undefined,
  onState?: (state: 'recording' | 'transcribing') => void,
  onLevel?: (level: number | null) => void,
) {
  const stream = await navigator.mediaDevices.getUserMedia({
    audio: deviceId ? { deviceId: { exact: deviceId } } : true,
  });
  const preferredMimeType = 'audio/webm;codecs=opus';
  const recorder = new MediaRecorder(
    stream,
    MediaRecorder.isTypeSupported(preferredMimeType)
      ? { mimeType: preferredMimeType }
      : undefined,
  );
  const chunks: Blob[] = [];
  const recordedAt = performance.now();
  const recording = new Promise<Blob>((resolve, reject) => {
    recorder.ondataavailable = (event) => {
      if (event.data.size === 0) return;
      chunks.push(event.data);
    };
    recorder.onerror = () => reject(new Error('Recording failed'));
    recorder.onstop = () =>
      resolve(new Blob(chunks, { type: recorder.mimeType }));
  });

  recorder.start();
  const levelMonitor = onLevel ? startMicLevelMonitor(stream, onLevel) : null;
  const ready = waitForCapturedAudioFrame(stream).then(() => {
    onState?.('recording');
  });
  return {
    ready,
    stop: async () => {
      onState?.('transcribing');
      levelMonitor?.stop();
      onLevel?.(null);
      recorder.stop();
      const blob = await recording;
      const track = stream.getAudioTracks()[0];
      const trackSampleRate = track?.getSettings()?.sampleRate;
      stream.getTracks().forEach((streamTrack) => streamTrack.stop());
      const mimeType = blob.type || recorder.mimeType || 'unknown';
      if (blob.size === 0 || blob.size > MAX_DICTATION_AUDIO_BYTES) {
        throw new Error('The dictation recording is empty or too large.');
      }
      return {
        audio: new Uint8Array(await blob.arrayBuffer()),
        mimeType,
        debug: {
          mimeType,
          blobBytes: blob.size,
          durationSeconds:
            Math.round(((performance.now() - recordedAt) / 1_000) * 100) / 100,
          trackSampleRate,
          trackMuted: track?.muted ?? null,
        },
      };
    },
    cancel: () => {
      levelMonitor?.stop();
      onLevel?.(null);
      recorder.stop();
      stream.getTracks().forEach((track) => track.stop());
    },
  };
}

const MIC_LEVEL_MONITOR_INTERVAL_MS = 100;
const MIC_LEVEL_WORKLET_BLOCK_FRAMES = 4096;

function startMicLevelMonitor(
  stream: MediaStream,
  onLevel: (level: number | null) => void,
) {
  const context = new AudioContext();
  const source = context.createMediaStreamSource(stream);
  const silentOutput = context.createGain();
  silentOutput.gain.value = 0;
  const monitor = {
    smoothed: 0,
    lastSent: -1,
    stopped: false,
  };
  function emit(peak: number) {
    if (monitor.stopped) return;
    monitor.smoothed = smoothMicLevel(monitor.smoothed, peak);
    const rounded = Math.round(monitor.smoothed * 100) / 100;
    if (rounded === monitor.lastSent) return;
    monitor.lastSent = rounded;
    onLevel(rounded);
  }

  let worklet: AudioWorkletNode | undefined;
  let fallbackTimer: number | undefined;
  function startIntervalFallback() {
    if (monitor.stopped || fallbackTimer !== undefined) return;
    const analyser = context.createAnalyser();
    analyser.fftSize = 512;
    source.connect(analyser);
    const samples = new Float32Array(analyser.fftSize);
    fallbackTimer = window.setInterval(() => {
      if (monitor.stopped) return;
      analyser.getFloatTimeDomainData(samples);
      let peak = 0;
      for (const sample of samples) {
        const absolute = Math.abs(sample);
        if (absolute > peak) peak = absolute;
      }
      emit(peak);
    }, MIC_LEVEL_MONITOR_INTERVAL_MS);
  }

  const workletUrl = URL.createObjectURL(
    new Blob(
      [
        `class MicLevelProcessor extends AudioWorkletProcessor {
  blockPeak = 0;
  blockFrames = 0;

  process(inputs) {
    const channel = inputs[0]?.[0];
    if (channel?.length) {
      for (const sample of channel) {
        this.blockPeak = Math.max(this.blockPeak, Math.abs(sample));
      }
      this.blockFrames += channel.length;
      if (this.blockFrames >= ${MIC_LEVEL_WORKLET_BLOCK_FRAMES}) {
        this.port.postMessage(this.blockPeak);
        this.blockPeak = 0;
        this.blockFrames = 0;
      }
    }
    return true;
  }
}
registerProcessor('mic-level', MicLevelProcessor);`,
      ],
      { type: 'text/javascript' },
    ),
  );
  try {
    void context.audioWorklet
      .addModule(workletUrl)
      .then(() => {
        if (monitor.stopped) return;
        worklet = new AudioWorkletNode(context, 'mic-level');
        worklet.port.onmessage = (event) => emit(Number(event.data) || 0);
        source
          .connect(worklet)
          .connect(silentOutput)
          .connect(context.destination);
      })
      .catch(() => startIntervalFallback());
  } catch {
    startIntervalFallback();
  }

  return {
    stop() {
      if (monitor.stopped) return;
      monitor.stopped = true;
      if (fallbackTimer !== undefined) window.clearInterval(fallbackTimer);
      URL.revokeObjectURL(workletUrl);
      source.disconnect();
      worklet?.disconnect();
      silentOutput.disconnect();
      void context.close().catch(() => {});
    },
  };
}

async function waitForCapturedAudioFrame(stream: MediaStream) {
  const track = stream.getAudioTracks()[0];
  if (!track) throw new Error('Microphone did not provide an audio track');
  const readiness = createMicrophoneReadinessTracker(
    needsBluetoothMicrophoneReadiness(
      track.label,
      track.getSettings().sampleRate,
    ),
  );
  const context = new AudioContext();
  const source = context.createMediaStreamSource(stream);
  const silentOutput = context.createGain();
  silentOutput.gain.value = 0;
  const workletUrl = URL.createObjectURL(
    new Blob(
      [
        `class DictationReadinessProcessor extends AudioWorkletProcessor {
  frameCount = 0;
  peak = 0;

  process(inputs) {
    const channel = inputs[0]?.[0];
    if (!channel?.length) return true;
    this.frameCount += channel.length;
    for (const sample of channel) this.peak = Math.max(this.peak, Math.abs(sample));
    if (this.frameCount >= 256) {
      this.port.postMessage(this.peak);
      this.frameCount = 0;
      this.peak = 0;
    }
    return true;
  }
}
registerProcessor('dictation-readiness', DictationReadinessProcessor);`,
      ],
      { type: 'text/javascript' },
    ),
  );
  let worklet: AudioWorkletNode | undefined;
  try {
    await context.audioWorklet.addModule(workletUrl);
    worklet = new AudioWorkletNode(context, 'dictation-readiness');
    source.connect(worklet).connect(silentOutput).connect(context.destination);
    if (context.state !== 'running') await context.resume();
    await new Promise<void>((resolve, reject) => {
      const timeout = window.setTimeout(
        () => finish(new Error('Microphone did not become ready')),
        MICROPHONE_READY_TIMEOUT_MS,
      );
      function finish(error?: Error) {
        window.clearTimeout(timeout);
        track.removeEventListener('ended', handleEnded);
        if (error) reject(error);
        else resolve();
      }
      function handleEnded() {
        finish(new Error('Microphone disconnected before it became ready'));
      }
      track.addEventListener('ended', handleEnded, { once: true });
      if (track.readyState === 'ended') {
        handleEnded();
        return;
      }
      worklet.port.onmessage = (event) => {
        if (
          track.readyState === 'live' &&
          track.enabled &&
          !track.muted &&
          context.state === 'running' &&
          readiness.consume(
            performance.now(),
            Number(event.data) > MICROPHONE_SIGNAL_THRESHOLD,
          )
        )
          finish();
      };
    });
  } finally {
    URL.revokeObjectURL(workletUrl);
    source.disconnect();
    worklet?.disconnect();
    silentOutput.disconnect();
    await context.close();
  }
}
