// biome-ignore-all lint: Extension AI attachments are dynamic extension-supplied payloads.
import fs from 'node:fs/promises';
import path from 'node:path';
import { trustedExtensionApiUnavailable } from '../extension-host/extension-context-capabilities';
import { fileToExtensionFile } from '../files/extension-file';
import {
  displayUserPath,
  expandUserPath,
  extensionForPath,
  isImagePath,
} from '../files/file-utils';
import { mimeTypeForPath } from '../files/mime-type';
import { ocrImage } from '../files/ocr';
import { limitedOutput } from '../platform/limited-output';
import { ExtensionAiDecisionInputError } from './extension-ai-decision-request';

const EXTENSION_AI_ATTACHMENT_LIMIT = 8;
const EXTENSION_AI_TEXT_ATTACHMENT_LIMIT = 80_000;
const EXTENSION_AI_IMAGE_ATTACHMENT_MAX_BYTES = 8 * 1024 * 1024;

function extensionAiDataUrlImage(dataUrl) {
  const match = String(dataUrl || '').match(/^data:([^;,]+);base64,(.*)$/s);
  if (!match) throw new Error('AI image attachment must be a base64 data URL');
  return { type: 'image' as const, mimeType: match[1], data: match[2] };
}

async function extensionAiPathImage(filePath) {
  const resolvedPath = expandUserPath(filePath);
  const stat = await fs.stat(resolvedPath);
  if (stat.size > EXTENSION_AI_IMAGE_ATTACHMENT_MAX_BYTES)
    throw new Error(
      `AI image attachment is too large: ${displayUserPath(resolvedPath)}`,
    );
  return {
    type: 'image' as const,
    mimeType: mimeTypeForPath(resolvedPath) || 'image/png',
    data: (await fs.readFile(resolvedPath)).toString('base64'),
  };
}

function isTextLikeAttachmentPath(filePath) {
  const mime = mimeTypeForPath(filePath);
  const ext = extensionForPath(filePath);
  return (
    mime.startsWith('text/') ||
    [
      'md',
      'markdown',
      'txt',
      'json',
      'csv',
      'tsv',
      'xml',
      'html',
      'css',
      'js',
      'ts',
      'tsx',
      'jsx',
      'yml',
      'yaml',
      'log',
    ].includes(ext)
  );
}

async function extensionAiFileContext(filePath, options: any = {}) {
  const resolvedPath = expandUserPath(filePath);
  const stat = await fs.stat(resolvedPath).catch(() => null);
  const title = options.title || path.basename(resolvedPath);
  if (!stat)
    return `### ${title}\nMissing file: ${displayUserPath(resolvedPath)}`;
  const metadata = await fileToExtensionFile(resolvedPath);
  const header = `### ${title}\nPath: ${metadata.displayPath}\nMIME: ${metadata.mimeType || 'unknown'}\nSize: ${metadata.size} bytes`;
  if (options.as === 'metadata') return header;
  if (!isTextLikeAttachmentPath(resolvedPath)) return header;
  const text = await fs.readFile(resolvedPath, 'utf8');
  return `${header}\n\n${limitedOutput(text, EXTENSION_AI_TEXT_ATTACHMENT_LIMIT)}`;
}

async function resolveExtensionAiAttachmentList(input, rejectExcess = false) {
  const output: any[] = [];
  async function visit(value) {
    const resolved = await value;
    if (resolved == null || resolved === false) return;
    if (Array.isArray(resolved)) {
      for (const item of resolved) await visit(item);
      return;
    }
    output.push(resolved);
  }
  await visit(input);
  if (rejectExcess && output.length > EXTENSION_AI_ATTACHMENT_LIMIT)
    throw new Error(
      `AI decisions accept at most ${EXTENSION_AI_ATTACHMENT_LIMIT} attachments`,
    );
  return output.slice(0, EXTENSION_AI_ATTACHMENT_LIMIT);
}

export function extensionAiOcrText(result) {
  if (typeof result === 'string') return result;
  if (Array.isArray(result?.blocks))
    return result.blocks
      .map((block) => block.text)
      .filter(Boolean)
      .join('\n');
  if (Array.isArray(result?.observations))
    return result.observations
      .map((item) => item.text || item.transcript)
      .filter(Boolean)
      .join('\n');
  return result?.text || result?.transcript || JSON.stringify(result);
}

export async function normalizeExtensionAiAttachments(
  extension,
  attachments,
  capabilities,
  options: { requireReadableFiles?: boolean } = {},
) {
  const textSections: string[] = [];
  const images: Array<{ type: 'image'; data: string; mimeType: string }> = [];
  for (const attachment of await resolveExtensionAiAttachmentList(
    attachments || [],
    options.requireReadableFiles,
  )) {
    if (typeof attachment === 'string') {
      textSections.push(
        limitedOutput(attachment, EXTENSION_AI_TEXT_ATTACHMENT_LIMIT),
      );
      continue;
    }
    const type =
      attachment?.type ||
      (attachment?.text == null
        ? attachment?.path || attachment?.file
          ? 'file'
          : attachment?.dataUrl || attachment?.imageDataUrl
            ? 'image'
            : ''
        : 'text');
    if (type === 'text') {
      textSections.push(
        `${attachment.title ? `### ${attachment.title}\n` : ''}${limitedOutput(attachment.text || '', EXTENSION_AI_TEXT_ATTACHMENT_LIMIT)}`,
      );
      continue;
    }
    if (type === 'image') {
      const source =
        attachment.dataUrl ||
        attachment.imageDataUrl ||
        attachment.data ||
        attachment.path ||
        attachment.file?.path ||
        attachment.filePath;
      if (
        attachment.data &&
        attachment.mimeType &&
        !String(source || '').startsWith('data:')
      )
        images.push({
          type: 'image' as const,
          data: String(attachment.data),
          mimeType: String(attachment.mimeType),
        });
      else if (String(source || '').startsWith('data:'))
        images.push(extensionAiDataUrlImage(source));
      else {
        if (!capabilities.files)
          throw trustedExtensionApiUnavailable('desktop.files');
        images.push(await extensionAiPathImage(source));
      }
      if (attachment.ocr) {
        if (!capabilities.ocr) throw trustedExtensionApiUnavailable('ocr');
        textSections.push(
          `### ${attachment.title || 'OCR'}\n${limitedOutput(extensionAiOcrText(await ocrImage(source)), EXTENSION_AI_TEXT_ATTACHMENT_LIMIT)}`,
        );
      }
      continue;
    }
    if (type === 'file') {
      if (!capabilities.files)
        throw trustedExtensionApiUnavailable('desktop.files');
      const filePath =
        attachment.path || attachment.file?.path || attachment.filePath;
      if (!filePath) {
        if (options.requireReadableFiles)
          throw new ExtensionAiDecisionInputError(
            'decision_attachment_unreadable',
            'Decision file attachment has no path.',
          );
        continue;
      }
      const resolvedPath = expandUserPath(filePath);
      if (options.requireReadableFiles) await fs.access(resolvedPath);
      const shouldAttachImage =
        (attachment.as === 'image' ||
          (!attachment.as && isImagePath(resolvedPath))) &&
        attachment.as !== 'text';
      if (
        options.requireReadableFiles &&
        !shouldAttachImage &&
        !attachment.ocr &&
        attachment.as !== 'ocr' &&
        (attachment.as === 'metadata' ||
          !isTextLikeAttachmentPath(resolvedPath))
      )
        throw new ExtensionAiDecisionInputError(
          'decision_attachment_unreadable',
          `Attachment ${path.basename(resolvedPath)} cannot be read as text or an image, so it was not sent.`,
        );
      if (shouldAttachImage)
        images.push(await extensionAiPathImage(resolvedPath));
      if (
        !shouldAttachImage ||
        attachment.as === 'text' ||
        attachment.as === 'metadata'
      )
        textSections.push(
          await extensionAiFileContext(resolvedPath, attachment),
        );
      if (attachment.ocr || attachment.as === 'ocr') {
        if (!capabilities.ocr) throw trustedExtensionApiUnavailable('ocr');
        textSections.push(
          `### ${attachment.title || `OCR ${path.basename(resolvedPath)}`}\n${limitedOutput(extensionAiOcrText(await ocrImage(resolvedPath)), EXTENSION_AI_TEXT_ATTACHMENT_LIMIT)}`,
        );
      }
      continue;
    }
    if (options.requireReadableFiles)
      throw new ExtensionAiDecisionInputError(
        'decision_attachment_unreadable',
        'Unsupported decision attachment.',
      );
  }
  return { context: textSections.filter(Boolean).join('\n\n'), images };
}

function normalizeExtensionAiModelRole(value) {
  if (value == null || value === '') return;
  if (value === 'smart' || value === 'fast') return value;
  throw new Error(
    `Unsupported AI model role: ${value}. Use 'smart' or 'fast'.`,
  );
}

export function normalizeExtensionAiCallOptions(input: any = {}) {
  if (typeof input === 'string')
    return { model: normalizeExtensionAiModelRole(input) };
  const output = { ...(input || {}) };
  const model = normalizeExtensionAiModelRole(input?.model);
  if (model) output.model = model;
  else delete output.model;
  return output;
}
