// biome-ignore-all lint: Extension AI options and attachments are dynamic extension-supplied payloads.
import path from 'node:path';
import { trustedExtensionApiUnavailable } from '../extension-host/extension-context-capabilities';
import { getNevermindAuth } from '../backend/nevermind-auth';
import { ocrImage } from '../files/ocr';
import {
  extensionAiOcrText,
  normalizeExtensionAiAttachments,
  normalizeExtensionAiCallOptions,
} from './extension-ai-attachments';
import { buildDecisionWireRequest } from './extension-ai-decision-request';
import { requestNevermindDecision } from './nevermind-decision-client';

export type ExtensionAiDeps = {
  checkAiRateLimit: (extension: any) => boolean;
  getNevermindAi: () => any;
  selectedText: () => Promise<string>;
  selectedExtensionFiles: () => Promise<unknown[]>;
  readDesktopClipboard: (options?: any) => Promise<unknown>;
};

export function createExtensionAiFactory(deps: ExtensionAiDeps) {
  const {
    checkAiRateLimit,
    getNevermindAi,
    selectedText,
    selectedExtensionFiles,
    readDesktopClipboard,
  } = deps;

  function createExtensionAi(extension) {
    const extensionKey = path
      .basename(extension.__filePath || extension.id || 'extension')
      .replace(/[^a-zA-Z0-9._-]/g, '-');
    // Trusted local extensions are not sandboxed by their declarations.
    const capabilities = { files: true, clipboard: true, ocr: true };
    const enforceAiQuota = () => {
      if (!checkAiRateLimit(extension))
        throw Object.assign(new Error('AI rate limit exceeded'), {
          code: 'ai-rate-limit-exceeded',
          extensionId: extension?.id,
        });
    };
    const normalizeOptions = async (options: any = {}) => {
      const callOptions = normalizeExtensionAiCallOptions(options);
      const normalized = await normalizeExtensionAiAttachments(
        extension,
        callOptions.attachments || [],
        capabilities,
      );
      return {
        preparationKey: extensionKey,
        model: callOptions.model,
        system: callOptions.system,
        signal: callOptions.signal,
        context: normalized.context,
        images: normalized.images,
        onEvent: (event) => {
          if (event.type === 'delta' && event.text)
            callOptions.onDelta?.(event.text);
          callOptions.onEvent?.(event);
        },
      };
    };
    const stream = (
      prompt,
      options: any = {},
      session: any = getNevermindAi(),
    ) => {
      enforceAiQuota();
      const callOptions = normalizeExtensionAiCallOptions(options);
      const controller = new AbortController();
      const removeExternalAbortListener = callOptions.signal?.addEventListener
        ? (() => {
            const listener = () => controller.abort();
            callOptions.signal.addEventListener('abort', listener, {
              once: true,
            });
            return () =>
              callOptions.signal.removeEventListener?.('abort', listener);
          })()
        : () => {};
      let inner: any = null;
      const result = (async () => {
        const normalized = await normalizeOptions({
          ...callOptions,
          signal: controller.signal,
        });
        inner = session.stream(String(prompt || ''), normalized);
        return inner.result;
      })().finally(removeExternalAbortListener);
      return {
        result,
        abort: () => {
          controller.abort();
          inner?.abort?.();
        },
      };
    };
    const ai: any = async (prompt, options: any = {}) =>
      ai.ask(prompt, options);
    ai.isAvailable = async () => Boolean(await getNevermindAuth());
    ai.ask = async (prompt, options: any = {}) => {
      enforceAiQuota();
      return getNevermindAi().ask(
        String(prompt || ''),
        await normalizeOptions(options),
      );
    };
    ai.decide = async (options: any = {}) => {
      enforceAiQuota();
      const normalized = await normalizeExtensionAiAttachments(
        extension,
        options.attachments || [],
        capabilities,
        { requireReadableFiles: true },
      );
      return requestNevermindDecision(
        buildDecisionWireRequest({
          input: String(options.input ?? ''),
          contextText: normalized.context,
          images: normalized.images,
          questions: options.questions ?? [],
        }),
        options.signal,
      );
    };
    ai.prepare = async (options: any = {}) =>
      getNevermindAi().prepare(await normalizeOptions(options));
    ai.stream = stream;
    ai.session = (id = 'default', options: any = {}) => {
      const sessionOptions = normalizeExtensionAiCallOptions(options);
      const session = getNevermindAi().session(
        `${extensionKey}:${String(id || 'default')}`,
        { system: sessionOptions.system, model: sessionOptions.model },
      );
      return {
        ...session,
        ask: async (prompt: any, askOptions: any = {}) => {
          enforceAiQuota();
          return session.ask(
            prompt,
            await normalizeOptions({
              ...sessionOptions,
              ...normalizeExtensionAiCallOptions(askOptions),
            }),
          );
        },
        stream: (prompt: any, streamOptions: any = {}) =>
          stream(
            prompt,
            {
              ...sessionOptions,
              ...normalizeExtensionAiCallOptions(streamOptions),
            },
            session,
          ),
      };
    };
    ai.attachments = {
      text: (text, title) => ({
        type: 'text',
        text: String(text || ''),
        title,
      }),
      image: (input, options: any = {}) => ({
        ...options,
        type: 'image',
        ...(String(input || '').startsWith('data:')
          ? { dataUrl: input }
          : { path: input }),
      }),
      file: (input, options: any = {}) => ({
        ...options,
        type: 'file',
        path:
          typeof input === 'string' ? input : input?.path || input?.filePath,
      }),
      selectedText: async (title = 'Selected Text') => ({
        type: 'text',
        title,
        text: await selectedText(),
      }),
      selectedFiles: async (options: any = {}) => {
        if (!capabilities.files)
          throw trustedExtensionApiUnavailable('desktop.files');
        return (await selectedExtensionFiles()).map((file) => ({
          ...options,
          type: 'file',
          file,
        }));
      },
      clipboard: async (options: any = {}) => {
        if (!capabilities.clipboard)
          throw trustedExtensionApiUnavailable('clipboard.history');
        const item: any = await readDesktopClipboard();
        if (item.type === 'text')
          return {
            type: 'text',
            title: options.title || 'Clipboard',
            text: item.text,
          };
        if (item.type === 'image')
          return {
            ...options,
            type: 'image',
            title: options.title || 'Clipboard Image',
            dataUrl: item.image,
          };
        if (item.type === 'files')
          return item.files.map((file) => ({ ...options, type: 'file', file }));
        return null;
      },
      ocrImage: async (input, options: any = {}) => {
        if (!capabilities.ocr) throw trustedExtensionApiUnavailable('ocr');
        return {
          type: 'text',
          title: options.title || 'OCR Text',
          text: extensionAiOcrText(await ocrImage(input, options)),
        };
      },
    };
    return ai;
  }

  return createExtensionAi;
}
