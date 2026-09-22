import { performance } from 'node:perf_hooks';

const OPERATION_ID_RADIX = 36;

interface DictationTiming {
  startedAt: number;
  stopRequestedAt?: number;
}

export type DictationRendererCommand =
  | {
      type: 'start';
      operationId: string;
      deviceId?: string;
    }
  | { type: 'stop'; operationId: string }
  | { type: 'release'; operationId: string }
  | { type: 'cancel'; operationId?: string }
  | { type: 'devices' };

export type DictationRendererReply =
  | { type: 'recording'; operationId: string }
  | {
      type: 'audio';
      operationId: string;
      audio: Uint8Array;
      mimeType: string;
      debug?: Record<string, unknown>;
    }
  | {
      type: 'devices';
      devices: Array<{ id: string; title: string; isDefault: boolean }>;
    }
  | { type: 'error'; message: string; operationId?: string };

export interface DictationService {
  status(): Promise<string>;
  apiAvailable(): Promise<boolean>;
  devices(): Promise<Array<{ id: string; title: string; isDefault: boolean }>>;
  start(options?: {
    deviceId?: string;
    muteSystemAudioWhileRecording?: boolean;
  }): Promise<void>;
  stop(): Promise<string>;
  cancel(): Promise<void>;
  dispose(): Promise<void>;
  reply(reply: DictationRendererReply): void;
}

// biome-ignore lint/complexity/noExcessiveLinesPerFunction: One stateful lifecycle owns each dictation operation.
export function createDictationService(
  send: (command: DictationRendererCommand) => void,
  dependencies: {
    muteSystemAudio?: () => Promise<{ restore(): Promise<void> }>;
    onSystemAudioError?: (error: unknown) => void;
    transcribeAudio?: (input: {
      operationId: string;
      audio: Uint8Array;
      mimeType: string;
      signal: AbortSignal;
    }) => Promise<string>;
    apiAvailable?: () => Promise<boolean>;
    recordTiming?: (
      name: string,
      durationMs: number,
      detail?: Record<string, unknown>,
    ) => void;
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
  let systemAudioMute: { restore(): Promise<void> } | null = null;
  let operationCounter = 0;
  let activeOperationId: string | null = null;
  let activeTiming: DictationTiming | null = null;
  let transcriptionAbort: AbortController | null = null;

  function recordTiming(
    name: string,
    startedAt: number,
    operationId: string,
    detail: Record<string, unknown> = {},
  ) {
    dependencies.recordTiming?.(name, performance.now() - startedAt, {
      operationId,
      ...detail,
      alwaysLog: true,
    });
  }

  async function restoreSystemAudio() {
    const mute = systemAudioMute;
    if (!mute) {
      return;
    }
    try {
      await mute.restore();
      if (systemAudioMute === mute) {
        systemAudioMute = null;
      }
    } catch (error) {
      dependencies.onSystemAudioError?.(error);
    }
  }

  function status() {
    return Promise.resolve(currentStatus);
  }

  function apiAvailable() {
    return (
      dependencies.apiAvailable?.().catch(() => false) ?? Promise.resolve(false)
    );
  }

  function start(
    options: {
      deviceId?: string;
      muteSystemAudioWhileRecording?: boolean;
    } = {},
  ) {
    if (currentStatus !== 'idle') {
      return pendingStart?.promise || Promise.resolve();
    }
    let resolveStart!: () => void;
    let rejectStart!: (error: Error) => void;
    const promise = new Promise<void>((resolve, reject) => {
      resolveStart = resolve;
      rejectStart = reject;
    });
    pendingStart = { promise, resolve: resolveStart, reject: rejectStart };
    currentStatus = 'recording';
    const operationId = `dictation-${Date.now().toString(OPERATION_ID_RADIX)}-${(++operationCounter).toString(OPERATION_ID_RADIX)}`;
    activeOperationId = operationId;
    activeTiming = { startedAt: performance.now() };
    const { muteSystemAudioWhileRecording, ...rendererOptions } = options;
    const command = { type: 'start' as const, operationId, ...rendererOptions };
    if (muteSystemAudioWhileRecording && dependencies.muteSystemAudio) {
      dependencies
        .muteSystemAudio()
        .then((mute) => {
          if (currentStatus !== 'recording') {
            return mute.restore();
          }
          systemAudioMute = mute;
          send(command);
        })
        .catch((error) => {
          currentStatus = 'idle';
          pendingStart?.reject(
            error instanceof Error ? error : new Error(String(error)),
          );
          pendingStart = null;
          activeOperationId = null;
          activeTiming = null;
        });
    } else {
      send(command);
    }
    return promise;
  }

  function stop() {
    if (currentStatus !== 'recording') {
      return Promise.reject(new Error('Dictation is not recording'));
    }
    const operationId = activeOperationId;
    if (!operationId) {
      currentStatus = 'idle';
      return Promise.reject(new Error('Dictation operation is unavailable'));
    }
    currentStatus = 'transcribing';
    if (activeTiming) {
      activeTiming.stopRequestedAt = performance.now();
    }
    const promise = new Promise<string>((resolve, reject) => {
      pendingStop = { resolve, reject };
    });
    restoreSystemAudio().finally(() => send({ type: 'stop', operationId }));
    return promise;
  }

  function cancel() {
    if (currentStatus === 'idle') {
      return Promise.resolve();
    }
    currentStatus = 'idle';
    transcriptionAbort?.abort();
    transcriptionAbort = null;
    pendingStart?.reject(new Error('Dictation cancelled'));
    pendingStart = null;
    pendingStop?.reject(new Error('Dictation cancelled'));
    pendingStop = null;
    const operationId = activeOperationId ?? undefined;
    activeOperationId = null;
    activeTiming = null;
    return restoreSystemAudio().finally(() =>
      send({ type: 'cancel', operationId }),
    );
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

  function reply(reply: DictationRendererReply) {
    if (reply.type === 'recording') {
      if (reply.operationId !== activeOperationId) {
        return;
      }
      pendingStart?.resolve();
      pendingStart = null;
      if (activeTiming) {
        recordTiming(
          'dictation.microphone-ready',
          activeTiming.startedAt,
          reply.operationId,
        );
      }
      return;
    }
    if (reply.type === 'audio') {
      transcribeCapturedAudio(reply);
      return;
    }
    if (reply.type === 'devices') {
      pendingDevices?.resolve(reply.devices);
      pendingDevices = null;
      return;
    }
    handleRendererError(reply);
  }

  function transcribeCapturedAudio(
    reply: Extract<DictationRendererReply, { type: 'audio' }>,
  ) {
    if (
      reply.operationId !== activeOperationId ||
      currentStatus !== 'transcribing'
    ) {
      return;
    }
    if (!dependencies.transcribeAudio) {
      failTranscription(
        reply.operationId,
        new Error('API dictation is unavailable'),
      );
      return;
    }
    const controller = new AbortController();
    transcriptionAbort = controller;
    const audioReceivedAt = performance.now();
    if (activeTiming?.stopRequestedAt) {
      recordTiming(
        'dictation.audio-finalize',
        activeTiming.stopRequestedAt,
        reply.operationId,
        { audioBytes: reply.audio.byteLength },
      );
    }
    dependencies
      .transcribeAudio({
        operationId: reply.operationId,
        audio: reply.audio,
        mimeType: reply.mimeType,
        signal: controller.signal,
      })
      .then((text) => {
        if (reply.operationId !== activeOperationId) {
          return;
        }
        recordTiming(
          'dictation.cloud-transcription',
          audioReceivedAt,
          reply.operationId,
          { transcriptLength: text.length },
        );
        finishTranscription(reply.operationId, text);
      })
      .catch((error) => {
        if (reply.operationId !== activeOperationId) {
          return;
        }
        failTranscription(
          reply.operationId,
          error instanceof Error ? error : new Error(String(error)),
        );
      })
      .finally(() => {
        if (transcriptionAbort === controller) {
          transcriptionAbort = null;
        }
      });
  }

  function handleRendererError(
    reply: Extract<DictationRendererReply, { type: 'error' }>,
  ) {
    if (reply.operationId && reply.operationId !== activeOperationId) {
      return;
    }
    currentStatus = 'idle';
    restoreSystemAudio();
    const error = new Error(reply.message);
    pendingStart?.reject(error);
    pendingStart = null;
    pendingStop?.reject(error);
    pendingDevices?.reject(error);
    pendingStop = null;
    pendingDevices = null;
    if (reply.operationId) {
      send({ type: 'release', operationId: reply.operationId });
    }
    activeOperationId = null;
    activeTiming = null;
  }

  function finishTranscription(operationId: string, text: string) {
    if (activeTiming?.stopRequestedAt) {
      recordTiming(
        'dictation.stop-to-transcript',
        activeTiming.stopRequestedAt,
        operationId,
        { transcriptLength: text.length },
      );
    }
    currentStatus = 'idle';
    restoreSystemAudio();
    pendingStop?.resolve(text);
    pendingStop = null;
    send({ type: 'release', operationId });
    activeOperationId = null;
    activeTiming = null;
  }

  function failTranscription(operationId: string, error: Error) {
    currentStatus = 'idle';
    pendingStop?.reject(error);
    pendingStop = null;
    send({ type: 'release', operationId });
    activeOperationId = null;
    activeTiming = null;
  }

  return {
    status,
    apiAvailable,
    devices,
    start,
    stop,
    cancel,
    dispose,
    reply,
  };
}
