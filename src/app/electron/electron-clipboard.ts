import type {
  Clipboard,
  ClipboardItem as ElectronClipboardItem,
  NativeImage,
} from 'electron';

export type MaybePromise<T> = T | PromiseLike<T>;

export type ClipboardBookmark = {
  title: string;
  url: string;
};

export type ClipboardWriteData = {
  text?: string;
  html?: string;
  rtf?: string;
  bookmark?: ClipboardBookmark;
  image?: NativeImage;
  buffers?: Record<string, Buffer>;
};

export type ClipboardApi = {
  clear: () => MaybePromise<void>;
  readText: () => MaybePromise<string>;
  readHTML: () => MaybePromise<string>;
  readRTF: () => MaybePromise<string>;
  readBookmark: () => MaybePromise<ClipboardBookmark>;
  readImage: () => MaybePromise<NativeImage>;
  readBuffer: (format: string) => MaybePromise<Buffer>;
  write: (data: ClipboardWriteData) => MaybePromise<void>;
  writeText: (text: string) => MaybePromise<void>;
  writeBuffer: (format: string, buffer: Buffer) => MaybePromise<void>;
  writeImage: (image: NativeImage) => MaybePromise<void>;
};

type ClipboardItemValue = string | Blob | ClipboardBookmark;
type ClipboardItemConstructor = new (
  items: Record<string, ClipboardItemValue>,
) => ElectronClipboardItem;

const BOOKMARK_MIME_TYPE = 'electron application/bookmark';
const RAW_FORMAT_PREFIX = 'electron application/osclipboard;format=';

function rawClipboardFormat(format: string) {
  return `${RAW_FORMAT_PREFIX}"${format}"`;
}

function readFormatsFor(format: string) {
  const formats = [rawClipboardFormat(format), format];
  if (format === 'public.file-url') formats.push('text/uri-list');
  return formats;
}

function blobForBuffer(buffer: Buffer, type?: string) {
  return new Blob([buffer], type ? { type } : undefined);
}

async function findClipboardItem(clipboard: Clipboard, formats: string[]) {
  const items = await clipboard.read();
  for (const item of items) {
    const format = formats.find((candidate) => item.types.includes(candidate));
    if (format) return { item, format };
  }
  return null;
}

async function readClipboardBuffer(clipboard: Clipboard, formats: string[]) {
  const found = await findClipboardItem(clipboard, formats);
  if (!found) return Buffer.alloc(0);
  const value = await found.item.getType(found.format);
  if (!value || typeof (value as Blob).arrayBuffer !== 'function')
    return Buffer.alloc(0);
  return Buffer.from(await (value as Blob).arrayBuffer());
}

export function createElectronClipboardApi(deps: {
  clipboard: Clipboard;
  nativeImage: {
    createFromBuffer: (buffer: Buffer) => NativeImage;
    createEmpty: () => NativeImage;
  };
  ClipboardItem: ClipboardItemConstructor;
}): ClipboardApi {
  async function readFormat(format: string) {
    return readClipboardBuffer(deps.clipboard, readFormatsFor(format)).then(
      (buffer) => buffer.toString('utf8'),
    );
  }

  async function readBookmark() {
    const found = await findClipboardItem(deps.clipboard, [BOOKMARK_MIME_TYPE]);
    if (!found) return { title: '', url: '' };
    const value = await found.item.getType(BOOKMARK_MIME_TYPE);
    if (
      value &&
      typeof value === 'object' &&
      'title' in value &&
      'url' in value
    )
      return {
        title: String(value.title || ''),
        url: String(value.url || ''),
      };
    return { title: '', url: '' };
  }

  async function readImage() {
    const buffer = await readClipboardBuffer(deps.clipboard, [
      'image/png',
      'image/jpeg',
    ]);
    return buffer.length
      ? deps.nativeImage.createFromBuffer(buffer)
      : deps.nativeImage.createEmpty();
  }

  function addBufferFormats(
    item: Record<string, ClipboardItemValue>,
    format: string,
    buffer: Buffer,
  ) {
    item[rawClipboardFormat(format)] = blobForBuffer(buffer, format);
    if (format === 'public.file-url')
      item['text/uri-list'] = blobForBuffer(buffer, 'text/uri-list');
  }

  async function write(data: ClipboardWriteData) {
    const item: Record<string, ClipboardItemValue> = {};
    if (data.text != null) item['text/plain'] = String(data.text);
    if (data.html != null) item['text/html'] = String(data.html);
    if (data.rtf != null) item['text/rtf'] = String(data.rtf);
    if (data.bookmark) item[BOOKMARK_MIME_TYPE] = data.bookmark;
    if (data.image)
      item['image/png'] = blobForBuffer(data.image.toPNG(), 'image/png');
    for (const [format, buffer] of Object.entries(data.buffers || {}))
      addBufferFormats(item, format, buffer);
    if (Object.keys(item).length === 0) return deps.clipboard.clear();
    await deps.clipboard.write([new deps.ClipboardItem(item)]);
  }

  return {
    clear: () => deps.clipboard.clear(),
    readText: () => deps.clipboard.readText(),
    readHTML: () => readFormat('text/html'),
    readRTF: () => readFormat('text/rtf'),
    readBookmark,
    readImage,
    readBuffer: (format) =>
      readClipboardBuffer(deps.clipboard, readFormatsFor(format)),
    write,
    writeText: (text) => deps.clipboard.writeText(text),
    writeBuffer: (format, buffer) => write({ buffers: { [format]: buffer } }),
    writeImage: (image) => write({ image }),
  };
}
