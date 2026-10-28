import type { IpcMain, SystemPreferences } from 'electron';
import type { DictationRendererReply } from '../dictation/dictation-service';
import { debug as loggerDebug } from '../platform/logger';

const MAX_DICTATION_AUDIO_BYTES = 4_194_304;

export type DictationIpcDeps = {
  ipcMain: Pick<IpcMain, 'handle' | 'on'>;
  systemPreferences: Pick<SystemPreferences, 'getMediaAccessStatus'>;
  paletteWindow: { win?: { webContents: unknown } | null };
  extensionWindowManager: {
    sendToIndicator: (...args: unknown[]) => unknown;
  };
  dictationService: {
    reply: (reply: DictationRendererReply) => unknown;
  };
};

export function registerDictationIpc({
  ipcMain,
  systemPreferences,
  paletteWindow,
  extensionWindowManager,
  dictationService,
}: DictationIpcDeps) {
  ipcMain.handle('microphone:get-status', () =>
    systemPreferences.getMediaAccessStatus('microphone'),
  );
  ipcMain.on('dictation:level', (event, level) => {
    if (event.sender !== paletteWindow.win?.webContents) return;
    const normalized =
      typeof level === 'number' && Number.isFinite(level)
        ? Math.min(1, Math.max(0, level))
        : null;
    extensionWindowManager.sendToIndicator(
      'nevermind.dictation',
      'dictation',
      'indicator:mic-level',
      normalized,
    );
  });
  ipcMain.on('dictation:reply', (event, reply) => {
    if (event.sender !== paletteWindow.win?.webContents) return;
    if (!reply || typeof reply !== 'object') return;
    if (reply.type === 'recording' && typeof reply.operationId === 'string') {
      dictationService.reply(reply as DictationRendererReply);
      return;
    }
    if (
      reply.type === 'audio' &&
      typeof reply.operationId === 'string' &&
      typeof reply.mimeType === 'string' &&
      reply.mimeType.toLowerCase().startsWith('audio/webm') &&
      Array.isArray(reply.segments) &&
      reply.segments.length > 0 &&
      reply.segments.length <= 120 &&
      reply.segments.every(
        (segment) =>
          segment instanceof Uint8Array &&
          segment.byteLength > 0 &&
          segment.byteLength <= MAX_DICTATION_AUDIO_BYTES,
      )
    ) {
      if (reply.debug && typeof reply.debug === 'object')
        loggerDebug('dictation.capture-stats', reply.debug, {
          source: 'renderer',
          scope: 'dictation',
        });
      dictationService.reply(reply as DictationRendererReply);
      return;
    }
    if (reply.type === 'audio' && typeof reply.operationId === 'string') {
      dictationService.reply({
        type: 'error',
        operationId: reply.operationId,
        message: 'The dictation recording is empty, unsupported, or too large.',
      });
      return;
    }
    if (reply.type === 'devices' && Array.isArray(reply.devices)) {
      dictationService.reply(reply as DictationRendererReply);
      return;
    }
    if (
      reply.type === 'error' &&
      typeof reply.message === 'string' &&
      (reply.operationId === undefined || typeof reply.operationId === 'string')
    )
      dictationService.reply(reply as DictationRendererReply);
  });
}
