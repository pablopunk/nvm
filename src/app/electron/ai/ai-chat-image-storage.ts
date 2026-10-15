// biome-ignore-all lint: AI chat messages and stored images are dynamic persisted payloads.
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileUrlForPath } from '../files/file-utils';
import { hashValue } from '../search/search-utils';
import type { NormalizedAiChatImage } from './ai-chat-images';

const AI_CHAT_IMAGE_STORAGE_MAX_BYTES = 512 * 1024 * 1024;

export type AiChatImageStorageDeps = {
  getAiChatImagesDir: () => string;
};

export function createAiChatImageStorage(deps: AiChatImageStorageDeps) {
  const { getAiChatImagesDir } = deps;

  let aiChatImageStoredBytes: number | null = null;
  let aiChatImageStorageMutation: Promise<void> = Promise.resolve();

  function aiChatImageDirectory(chatId) {
    return path.join(getAiChatImagesDir(), hashValue(String(chatId)));
  }

  function mutateAiChatImageStorage<T>(operation: () => Promise<T>) {
    const result = aiChatImageStorageMutation.catch(() => {}).then(operation);
    aiChatImageStorageMutation = result.then(
      () => undefined,
      () => undefined,
    );
    return result;
  }

  async function directoryFileBytes(directory: string): Promise<number> {
    const entries = await fs
      .readdir(directory, { withFileTypes: true })
      .catch(() => []);
    let total = 0;
    for (const entry of entries) {
      const entryPath = path.join(directory, entry.name);
      if (entry.isDirectory()) total += await directoryFileBytes(entryPath);
      else if (entry.isFile())
        total += await fs
          .stat(entryPath)
          .then((stat) => stat.size)
          .catch(() => 0);
    }
    return total;
  }

  async function currentAiChatImageStoredBytes() {
    aiChatImageStoredBytes ??= await directoryFileBytes(getAiChatImagesDir());
    return aiChatImageStoredBytes;
  }

  async function persistAiChatImages(
    chatId: string,
    images: NormalizedAiChatImage[],
  ) {
    if (!images.length) return [];
    return mutateAiChatImageStorage(async () => {
      const addedBytes = images.reduce(
        (total, image) => total + image.bytes.length,
        0,
      );
      const storedBytes = await currentAiChatImageStoredBytes();
      if (storedBytes + addedBytes > AI_CHAT_IMAGE_STORAGE_MAX_BYTES)
        throw new Error(
          'AI chat image storage is full. Remove chats with images and try again.',
        );
      const directory = aiChatImageDirectory(chatId);
      await fs.mkdir(directory, { recursive: true, mode: 0o700 });
      const writtenPaths: string[] = [];
      try {
        const stored = [] as Array<{
          path: string;
          mimeType: string;
          name?: string;
        }>;
        for (const image of images) {
          const imagePath = path.join(
            directory,
            `${crypto.randomUUID()}.${image.extension}`,
          );
          await fs.writeFile(imagePath, image.bytes, { mode: 0o600 });
          writtenPaths.push(imagePath);
          stored.push({
            path: imagePath,
            mimeType: image.mimeType,
            ...(image.name ? { name: image.name } : {}),
          });
        }
        aiChatImageStoredBytes = storedBytes + addedBytes;
        return stored;
      } catch (error) {
        await Promise.allSettled(
          writtenPaths.map((imagePath) => fs.rm(imagePath)),
        );
        aiChatImageStoredBytes = null;
        throw error;
      }
    });
  }

  async function removeAiChatImagePaths(imagePaths: string[]) {
    if (!imagePaths.length) return;
    await mutateAiChatImageStorage(async () => {
      const storedBytes = await currentAiChatImageStoredBytes();
      let removedBytes = 0;
      for (const imagePath of imagePaths) {
        const size = await fs
          .stat(imagePath)
          .then((stat) => stat.size)
          .catch(() => 0);
        try {
          await fs.rm(imagePath, { force: true });
          removedBytes += size;
        } catch {}
      }
      aiChatImageStoredBytes = Math.max(0, storedBytes - removedBytes);
    });
  }

  async function removeAiChatImageDirectory(chatId: string) {
    const directory = aiChatImageDirectory(chatId);
    await mutateAiChatImageStorage(async () => {
      const storedBytes = await currentAiChatImageStoredBytes();
      const removedBytes = await directoryFileBytes(directory);
      await fs.rm(directory, { recursive: true, force: true });
      aiChatImageStoredBytes = Math.max(0, storedBytes - removedBytes);
    });
  }

  function aiChatMessagesForRenderer(messages: any[] = []) {
    return messages.map(aiChatMessageForRenderer);
  }

  function aiChatMessageForRenderer(message) {
    return {
      role: message.role,
      content: String(message.content || ''),
      ...(Array.isArray(message.images)
        ? {
            images: message.images.flatMap((image) => {
              const imagePath = storedAiChatImagePath(image);
              if (!imagePath) return [];
              return [
                {
                  url: fileUrlForPath(imagePath),
                  ...(typeof image.name === 'string'
                    ? { alt: image.name }
                    : {}),
                },
              ];
            }),
          }
        : {}),
    };
  }

  function storedAiChatImagePath(image) {
    if (!(image && typeof image.path === 'string')) return null;
    const imagePath = path.resolve(image.path);
    const relative = path.relative(
      path.resolve(getAiChatImagesDir()),
      imagePath,
    );
    return relative.startsWith('..') || path.isAbsolute(relative)
      ? null
      : imagePath;
  }

  function removeStoredAiChatMessageImages(messages) {
    const imagePaths = messages.flatMap((message) =>
      Array.isArray(message.images)
        ? message.images.flatMap((image) => {
            const imagePath = storedAiChatImagePath(image);
            return imagePath ? [imagePath] : [];
          })
        : [],
    );
    if (!imagePaths.length) return;
    void removeAiChatImagePaths(imagePaths);
  }

  return {
    persistAiChatImages,
    removeAiChatImageDirectory,
    aiChatMessagesForRenderer,
    aiChatMessageForRenderer,
    storedAiChatImagePath,
    removeStoredAiChatMessageImages,
  };
}
