import { fromHub, type ParakeetModel } from 'parakeet.js';

const MODEL_ID = 'parakeet-tdt-0.6b-v3';
const MODEL_REPO_ID = 'ysdede/parakeet-tdt-0.6b-v3-onnx';
const MODEL_REVISION = 'main';
const MODEL_CACHE_DB_NAME = 'parakeet-cache-db';
const MODEL_CACHE_STORE_NAME = 'file-store';
type DictationBackend = 'webgpu' | 'wasm';

type DictationGpu = {
  requestAdapter(options?: {
    powerPreference?: 'low-power' | 'high-performance';
  }): Promise<unknown>;
};

const MODEL_CACHE_FILES: Record<DictationBackend, readonly string[]> = {
  wasm: [
    'encoder-model.int8.onnx',
    'decoder_joint-model.int8.onnx',
    'vocab.txt',
  ],
  webgpu: [
    'encoder-model.onnx',
    'encoder-model.onnx.data',
    'decoder_joint-model.int8.onnx',
    'vocab.txt',
  ],
};
const SAMPLE_RATE = 16_000;
const DEFAULT_MODEL_KEEP_ALIVE_MS = 5 * 60 * 1000;

let modelPromise: Promise<ParakeetModel> | null = null;
let backendPromise: Promise<DictationBackend> | null = null;
let loadedModel:
  | (ParakeetModel & {
      dispose?: () => Promise<void> | void;
    })
  | null = null;
let modelEvictionTimer: number | undefined;

function modelCacheKey(filename: string) {
  return `hf-${MODEL_REPO_ID}-${MODEL_REVISION}--${filename}`;
}

function getDictationBackend() {
  if (!backendPromise) {
    backendPromise = (async () => {
      const gpu = (navigator as Navigator & { gpu?: DictationGpu }).gpu;
      if (!gpu) return 'wasm';
      try {
        return (await gpu.requestAdapter({
          powerPreference: 'high-performance',
        }))
          ? 'webgpu'
          : 'wasm';
      } catch {
        return 'wasm';
      }
    })();
  }
  return backendPromise;
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
  const backend = await getDictationBackend();
  const database = await openModelCacheDatabase();
  if (!database) return false;
  const cacheFiles = MODEL_CACHE_FILES[backend];

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
      let remaining = cacheFiles.length;
      let cached = true;
      transaction.onerror = () => finish(false);
      for (const filename of cacheFiles) {
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
    modelPromise = getDictationBackend()
      .then(async (backend) => {
        try {
          return await loadModelForBackend(backend);
        } catch (error) {
          if (backend !== 'webgpu') throw error;
          backendPromise = Promise.resolve('wasm');
          return loadModelForBackend('wasm');
        }
      })
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

function loadModelForBackend(backend: DictationBackend) {
  return fromHub(MODEL_ID, {
    backend,
    encoderQuant: backend === 'webgpu' ? 'fp32' : 'int8',
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
) {
  const stream = await navigator.mediaDevices.getUserMedia({
    audio: deviceId ? { deviceId: { exact: deviceId } } : true,
  });
  const recorder = new MediaRecorder(stream);
  const chunks: Blob[] = [];
  const recording = new Promise<Blob>((resolve, reject) => {
    recorder.ondataavailable = (event) => {
      if (event.data.size > 0) chunks.push(event.data);
    };
    recorder.onerror = () => reject(new Error('Recording failed'));
    recorder.onstop = () =>
      resolve(new Blob(chunks, { type: recorder.mimeType }));
  });

  onState?.('recording');
  recorder.start();
  const modelLoadPromise = loadDictationModel().then((model) => {
    scheduleModelEviction(modelKeepAliveMs);
    return model;
  });
  void modelLoadPromise.catch(() => {});

  return {
    stop: async () => {
      onState?.('transcribing');
      recorder.stop();
      const blob = await recording;
      stream.getTracks().forEach((track) => track.stop());
      const audio = await decodeAudio(blob);
      if (!loadedModel) onState?.('loading-model');
      const model = await modelLoadPromise;
      const result = await model.transcribe(audio, SAMPLE_RATE);
      scheduleModelEviction(modelKeepAliveMs);
      return result.utterance_text.trim();
    },
    cancel: () => {
      void modelLoadPromise.catch(() => {});
      recorder.stop();
      stream.getTracks().forEach((track) => track.stop());
    },
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
