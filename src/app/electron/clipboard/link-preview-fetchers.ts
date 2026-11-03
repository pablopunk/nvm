import { fetchPublicHttp } from '../platform/public-http-fetch';

const MaxHtmlBytes = 256 * 1024;
const MaxImageBytes = 2 * 1024 * 1024;
const MaxCardImageWidth = 480;
const FetchTimeoutMs = 6000;
const RasterImageTypes = /^image\/(png|jpe?g|gif|webp)\b/i;

type PreviewNativeImage = {
  isEmpty: () => boolean;
  getSize: () => { width: number; height: number };
  resize: (options: {
    width: number;
    quality?: 'good' | 'better' | 'best';
  }) => PreviewNativeImage;
  toJPEG: (quality: number) => Buffer;
  toPNG: () => Buffer;
};

export type LinkPreviewFetcherDeps = {
  nativeImage: { createFromBuffer: (buffer: Buffer) => PreviewNativeImage };
};

/** Production ports for link previews: public-address-only fetches with bounded bodies. */
export function createLinkPreviewFetchers(deps: LinkPreviewFetcherDeps) {
  async function fetchHtml(url: string) {
    const response = await fetchPublicHttp(url, {
      accept: 'text/html,application/xhtml+xml',
      maxBytes: MaxHtmlBytes,
      timeoutMs: FetchTimeoutMs,
      isComplete: (text) => /<\/head>/i.test(text),
    });
    if (!(response.status >= 200 && response.status < 300)) return null;
    if (!/html/i.test(response.contentType)) return null;
    return {
      html: response.body.toString('utf8'),
      finalUrl: response.finalUrl,
    };
  }

  /** Decodes and downsizes in the host so the renderer never contacts the image's origin. */
  async function fetchImage(url: string) {
    const response = await fetchPublicHttp(url, {
      accept: 'image/png,image/jpeg,image/gif,image/webp',
      maxBytes: MaxImageBytes,
      timeoutMs: FetchTimeoutMs,
    });
    if (!(response.status >= 200 && response.status < 300)) return null;
    if (!RasterImageTypes.test(response.contentType)) return null;
    let image = deps.nativeImage.createFromBuffer(response.body);
    if (image.isEmpty()) return null;
    if (image.getSize().width > MaxCardImageWidth)
      image = image.resize({ width: MaxCardImageWidth, quality: 'good' });
    const isJpeg = /jpe?g/i.test(response.contentType);
    const encoded = isJpeg ? image.toJPEG(85) : image.toPNG();
    return `data:${isJpeg ? 'image/jpeg' : 'image/png'};base64,${encoded.toString('base64')}`;
  }

  return { fetchHtml, fetchImage };
}
