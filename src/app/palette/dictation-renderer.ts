import { fromHub, type ParakeetModel } from 'parakeet.js';
import { smoothMicLevel } from './mic-level';
import {
  createMicrophoneReadinessTracker,
  needsBluetoothMicrophoneReadiness,
} from './dictation-readiness';

const MODEL_ID = 'parakeet-tdt-0.6b-v3';
const MODEL_REPO_ID = 'ysdede/parakeet-tdt-0.6b-v3-onnx';
const MODEL_REVISION = 'main';
const MODEL_CACHE_DB_NAME = 'parakeet-cache-db';
const MODEL_CACHE_STORE_NAME = 'file-store';
const MODEL_CACHE_FILES = [
  'encoder-model.int8.onnx',
  'decoder_joint-model.int8.onnx',
  'vocab.txt',
];
const SAMPLE_RATE = 16_000;
const MICROPHONE_READY_TIMEOUT_MS = 7_000;
const MICROPHONE_SIGNAL_THRESHOLD = 0.000_01;
const DEFAULT_MODEL_KEEP_ALIVE_MS = 5 * 60 * 1000;

let modelPromise: Promise<ParakeetModel> | null = null;
let loadedModel:
  | (ParakeetModel & {
      dispose?: () => Promise<void> | void;
    })
  | null = null;
let modelEvictionTimer: number | undefined;

function modelCacheKey(filename: string) {
  return `hf-${MODEL_REPO_ID}-${MODEL_REVISION}--${filename}`;
}

function openModelCacheDatabase() {
  if (typeof indexedDB === 'undefined') return Promise.resolve(null);
  return new Promise<IDBDatabase | null>((resolve) => {
    const request = indexedDB.open(MODEL_CACHE_DB_NAME, 1);
    request.onupgradeneeded = () => {
      const database = request.result;
      if (!database.objectStoreNames.contains(MODEL_CACHE_STORE_NAME))
        database.createObjectStore(MODEL_CACHE_STORE_NAME);
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => resolve(null);
  });
}

export async function isDictationModelCached() {
  const database = await openModelCacheDatabase();
  if (!database) return false;

  return new Promise<boolean>((resolve) => {
    let settled = false;
    const finish = (cached: boolean) => {
      if (settled) return;
      settled = true;
      database.close();
      resolve(cached);
    };

    try {
      if (!database.objectStoreNames.contains(MODEL_CACHE_STORE_NAME)) {
        finish(false);
        return;
      }
      const transaction = database.transaction(
        [MODEL_CACHE_STORE_NAME],
        'readonly',
      );
      const store = transaction.objectStore(MODEL_CACHE_STORE_NAME);
      let remaining = MODEL_CACHE_FILES.length;
      let cached = true;
      transaction.onerror = () => finish(false);
      for (const filename of MODEL_CACHE_FILES) {
        const request = store.get(modelCacheKey(filename));
        request.onsuccess = () => {
          if (!request.result) cached = false;
          remaining -= 1;
          if (remaining === 0) finish(cached);
        };
        request.onerror = () => finish(false);
      }
    } catch {
      finish(false);
    }
  });
}

export function loadDictationModel() {
  if (modelEvictionTimer !== undefined) {
    window.clearTimeout(modelEvictionTimer);
    modelEvictionTimer = undefined;
  }
  if (!modelPromise) {
    modelPromise = loadModel()
      .then((model) => {
        loadedModel = model;
        return model;
      })
      .catch((error) => {
        modelPromise = null;
        loadedModel = null;
        throw error;
      });
  }
  return modelPromise;
}

function loadModel() {
  return fromHub(MODEL_ID, {
    backend: 'wasm',
    encoderQuant: 'int8',
    decoderQuant: 'int8',
    preprocessorBackend: 'js',
  });
}

export async function prepareDictationModel(
  modelKeepAliveMs = DEFAULT_MODEL_KEEP_ALIVE_MS,
) {
  await loadDictationModel();
  scheduleModelEviction(modelKeepAliveMs);
}

function scheduleModelEviction(keepAliveMs: number) {
  if (modelEvictionTimer !== undefined) window.clearTimeout(modelEvictionTimer);
  if (keepAliveMs < 0) return;
  modelEvictionTimer = window.setTimeout(() => {
    const model = loadedModel;
    modelPromise = null;
    loadedModel = null;
    modelEvictionTimer = undefined;
    void model?.dispose?.();
  }, keepAliveMs);
}

export async function recordDictation(
  deviceId: string | undefined,
  onState?: (state: 'recording' | 'transcribing' | 'loading-model') => void,
  modelKeepAliveMs = DEFAULT_MODEL_KEEP_ALIVE_MS,
  onLevel?: (level: number | null) => void,
) {
  const stream = await navigator.mediaDevices.getUserMedia({
    audio: deviceId ? { deviceId: { exact: deviceId } } : true,
  });
  const recorder = new MediaRecorder(stream);
  const chunks: Blob[] = [];
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
  const modelLoadPromise = loadDictationModel().then((model) => {
    scheduleModelEviction(modelKeepAliveMs);
    return model;
  });
  void modelLoadPromise.catch(() => {});

  return {
    ready,
    stop: async () => {
      onState?.('transcribing');
      levelMonitor?.stop();
      onLevel?.(null);
      recorder.stop();
      const blob = await recording;
      const track = stream.getAudioTracks()[0];
      const trackLabel = track?.label || 'unknown';
      const trackSampleRate = track?.getSettings()?.sampleRate;
      stream.getTracks().forEach((streamTrack) => streamTrack.stop());
      const mimeType = blob.type || recorder.mimeType || 'unknown';
      const audio = await decodeAudio(blob);
      const { peak, rms } = audioCaptureStats(audio);
      if (!loadedModel) onState?.('loading-model');
      const model = await modelLoadPromise;
      const transcribeStartedAt = performance.now();
      const result = await model.transcribe(audio, SAMPLE_RATE);
      scheduleModelEviction(modelKeepAliveMs);
      return {
        text: result.utterance_text.trim(),
        debug: {
          mimeType,
          blobBytes: blob.size,
          decodedFrames: audio.length,
          durationSeconds: Math.round((audio.length / SAMPLE_RATE) * 100) / 100,
          peak: Math.round(peak * 1_000_000) / 1_000_000,
          rms: Math.round(rms * 1_000_000) / 1_000_000,
          transcribeMs:
            Math.round((performance.now() - transcribeStartedAt) * 100) / 100,
          trackLabel,
          trackSampleRate,
          trackMuted: track?.muted ?? null,
        },
      };
    },
    cancel: () => {
      levelMonitor?.stop();
      onLevel?.(null);
      void modelLoadPromise.catch(() => {});
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

function audioCaptureStats(samples: Float32Array) {
  let peak = 0;
  let sumSquares = 0;
  for (const sample of samples) {
    const absolute = Math.abs(sample);
    if (absolute > peak) peak = absolute;
    sumSquares += sample * sample;
  }
  return {
    peak,
    rms: samples.length ? Math.sqrt(sumSquares / samples.length) : 0,
  };
}

async function decodeAudio(blob: Blob) {
  const context = new AudioContext({ sampleRate: SAMPLE_RATE });
  try {
    const decoded = await context.decodeAudioData(await blob.arrayBuffer());
    const channel = decoded.getChannelData(0);
    if (decoded.sampleRate === SAMPLE_RATE) return channel;
    const frameCount = Math.ceil(
      (channel.length * SAMPLE_RATE) / decoded.sampleRate,
    );
    const offline = new OfflineAudioContext(1, frameCount, SAMPLE_RATE);
    const source = offline.createBufferSource();
    const buffer = offline.createBuffer(1, channel.length, decoded.sampleRate);
    buffer.copyToChannel(channel, 0);
    source.buffer = buffer;
    source.connect(offline.destination);
    source.start();
    return (await offline.startRendering()).getChannelData(0);
  } finally {
    await context.close();
  }
}
