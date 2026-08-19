export type DictationRendererCommand =
  | { type: 'start'; deviceId?: string; modelKeepAliveMs?: number }
  | { type: 'stop' }
  | { type: 'cancel' }
  | { type: 'devices' }
  | { type: 'model-cache-status' }
  | { type: 'prepare-model'; modelKeepAliveMs?: number };

export type DictationRendererReply =
  | { type: 'recording' }
  | { type: 'result'; text: string }
  | {
      type: 'devices';
      devices: Array<{ id: string; title: string; isDefault: boolean }>;
    }
  | { type: 'model-cache-status'; cached: boolean }
  | { type: 'model-ready' }
  | { type: 'error'; message: string };

export type DictationModelCacheStatus = 'cached' | 'missing';

export type DictationService = {
  status(): Promise<string>;
  devices(): Promise<Array<{ id: string; title: string; isDefault: boolean }>>;
  modelCacheStatus(): Promise<DictationModelCacheStatus>;
  prepareModel(options?: { modelKeepAliveMs?: number }): Promise<void>;
  start(options?: {
    deviceId?: string;
    modelKeepAliveMs?: number;
    muteSystemAudioWhileRecording?: boolean;
  }): Promise<void>;
  stop(): Promise<string>;
  cancel(): Promise<void>;
  dispose(): Promise<void>;
  reply(reply: DictationRendererReply): void;
};

export function createDictationService(
  send: (command: DictationRendererCommand) => void,
  dependencies: {
    muteSystemAudio?: () => Promise<{ restore(): Promise<void> }>;
    onSystemAudioError?: (error: unknown) => void;
  } = {},
): DictationService {
  let currentStatus = 'idle';
  let pendingStart: {
    promise: Promise<void>;
    resolve: () => void;
    reject: (error: Error) => void;
  } | null = null;
  let pendingStop: {
    resolve: (text: string) => void;
    reject: (error: Error) => void;
  } | null = null;
  let pendingDevices: {
    resolve: (
      devices: Array<{ id: string; title: string; isDefault: boolean }>,
    ) => void;
    reject: (error: Error) => void;
  } | null = null;
  let pendingModelCacheStatus: {
    promise: Promise<DictationModelCacheStatus>;
    resolve: (status: DictationModelCacheStatus) => void;
    reject: (error: Error) => void;
  } | null = null;
  let pendingModelPreparation: {
    promise: Promise<void>;
    resolve: () => void;
    reject: (error: Error) => void;
  } | null = null;
  let systemAudioMute: { restore(): Promise<void> } | null = null;

  async function restoreSystemAudio() {
    const mute = systemAudioMute;
    if (!mute) return;
    try {
      await mute.restore();
      if (systemAudioMute === mute) systemAudioMute = null;
    } catch (error) {
      dependencies.onSystemAudioError?.(error);
    }
  }

  function status() {
    return Promise.resolve(currentStatus);
  }

  function start(
    options: {
      deviceId?: string;
      modelKeepAliveMs?: number;
      muteSystemAudioWhileRecording?: boolean;
    } = {},
  ) {
    if (currentStatus !== 'idle')
      return pendingStart?.promise || Promise.resolve();
    let resolveStart!: () => void;
    let rejectStart!: (error: Error) => void;
    const promise = new Promise<void>((resolve, reject) => {
      resolveStart = resolve;
      rejectStart = reject;
    });
    pendingStart = { promise, resolve: resolveStart, reject: rejectStart };
    currentStatus = 'recording';
    const { muteSystemAudioWhileRecording, ...rendererOptions } = options;
    if (muteSystemAudioWhileRecording && dependencies.muteSystemAudio) {
      void dependencies
        .muteSystemAudio()
        .then((mute) => {
          if (currentStatus !== 'recording') return mute.restore();
          systemAudioMute = mute;
          send({ type: 'start', ...rendererOptions });
        })
        .catch((error) => {
          currentStatus = 'idle';
          pendingStart?.reject(
            error instanceof Error ? error : new Error(String(error)),
          );
          pendingStart = null;
        });
    } else {
      send({ type: 'start', ...rendererOptions });
    }
    return promise;
  }

  function stop() {
    if (currentStatus !== 'recording')
      return Promise.reject(new Error('Dictation is not recording'));
    currentStatus = 'transcribing';
    const promise = new Promise<string>((resolve, reject) => {
      pendingStop = { resolve, reject };
    });
    void restoreSystemAudio().finally(() => send({ type: 'stop' }));
    return promise;
  }

  function cancel() {
    if (currentStatus === 'idle') return Promise.resolve();
    currentStatus = 'idle';
    pendingStart?.reject(new Error('Dictation cancelled'));
    pendingStart = null;
    pendingStop?.reject(new Error('Dictation cancelled'));
    pendingStop = null;
    return restoreSystemAudio().finally(() => send({ type: 'cancel' }));
  }

  function dispose() {
    return restoreSystemAudio();
  }

  function devices() {
    send({ type: 'devices' });
    return new Promise<
      Array<{ id: string; title: string; isDefault: boolean }>
    >((resolve, reject) => {
      pendingDevices = { resolve, reject };
    });
  }

  function modelCacheStatus() {
    if (pendingModelCacheStatus) return pendingModelCacheStatus.promise;
    let resolvePromise!: (status: DictationModelCacheStatus) => void;
    let rejectPromise!: (error: Error) => void;
    const promise = new Promise<DictationModelCacheStatus>(
      (resolve, reject) => {
        resolvePromise = resolve;
        rejectPromise = reject;
      },
    );
    pendingModelCacheStatus = {
      promise,
      resolve: resolvePromise,
      reject: rejectPromise,
    };
    send({ type: 'model-cache-status' });
    return promise;
  }

  function prepareModel(options: { modelKeepAliveMs?: number } = {}) {
    if (pendingModelPreparation) return pendingModelPreparation.promise;
    let resolvePromise!: () => void;
    let rejectPromise!: (error: Error) => void;
    const promise = new Promise<void>((resolve, reject) => {
      resolvePromise = resolve;
      rejectPromise = reject;
    });
    pendingModelPreparation = {
      promise,
      resolve: resolvePromise,
      reject: rejectPromise,
    };
    send({ type: 'prepare-model', ...options });
    return promise;
  }

  function reply(reply: DictationRendererReply) {
    if (reply.type === 'recording') {
      pendingStart?.resolve();
      pendingStart = null;
      return;
    }
    if (reply.type === 'result') {
      currentStatus = 'idle';
      void restoreSystemAudio();
      pendingStop?.resolve(reply.text);
      pendingStop = null;
      return;
    }
    if (reply.type === 'devices') {
      pendingDevices?.resolve(reply.devices);
      pendingDevices = null;
      return;
    }
    if (reply.type === 'model-cache-status') {
      pendingModelCacheStatus?.resolve(reply.cached ? 'cached' : 'missing');
      pendingModelCacheStatus = null;
      return;
    }
    if (reply.type === 'model-ready') {
      pendingModelPreparation?.resolve();
      pendingModelPreparation = null;
      return;
    }
    currentStatus = 'idle';
    void restoreSystemAudio();
    const error = new Error(reply.message);
    pendingStart?.reject(error);
    pendingStart = null;
    pendingStop?.reject(error);
    pendingDevices?.reject(error);
    pendingStop = null;
    pendingDevices = null;
    pendingModelCacheStatus?.reject(error);
    pendingModelCacheStatus = null;
    pendingModelPreparation?.reject(error);
    pendingModelPreparation = null;
  }

  return {
    status,
    devices,
    modelCacheStatus,
    prepareModel,
    start,
    stop,
    cancel,
    dispose,
    reply,
  };
}
