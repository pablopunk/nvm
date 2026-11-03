import net from 'node:net';
import { isPublicAddress } from '../platform/public-http-fetch';

export type LinkPreview = {
  url: string;
  status: 'loading' | 'ready' | 'unavailable';
  title?: string;
  description?: string;
  siteName?: string;
  image?: string;
};

export type LinkPreviewDeps = {
  /** Returns the first bytes of an HTML document, or null when it is not fetchable HTML. */
  fetchHtml: (
    url: string,
  ) => Promise<{ html: string; finalUrl: string } | null>;
  /** Returns a small, self-contained image data URL for a preview image, or null. */
  fetchImage?: (url: string) => Promise<string | null>;
  /** Notified when a background load settles so open views can refresh in place. */
  onSettled?: (url: string) => void;
  cacheLimit?: number;
};

const DefaultCacheLimit = 200;
/** Returns the normalized URL when the whole clipboard text is exactly one public http(s) URL. */
export function singleWebUrl(text: unknown): string | null {
  if (typeof text !== 'string') return null;
  const trimmed = text.trim();
  if (!trimmed || /\s/.test(trimmed)) return null;
  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    return null;
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null;
  const host = parsed.hostname.toLowerCase();
  const bareHost = host.replace(/^\[|\]$/g, '');
  if (net.isIP(bareHost)) return isPublicAddress(bareHost) ? parsed.href : null;
  if (host === 'localhost' || host.endsWith('.local') || !host.includes('.'))
    return null;
  return parsed.href;
}

function decodeEntities(value: string) {
  return value
    .replace(/&#(\d+);/g, (_, code) => String.fromCodePoint(Number(code)))
    .replace(/&#x([0-9a-f]+);/gi, (_, code) =>
      String.fromCodePoint(Number.parseInt(code, 16)),
    )
    .replace(/&quot;/g, '"')
    .replace(/&apos;|&#39;/g, "'")
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&')
    .replace(/\s+/g, ' ')
    .trim();
}

const AttributePattern =
  /([^\s"'<>/=]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>`]+)))?/y;

/** Reads one tag's attributes, treating `>` inside quoted values as text. */
function readAttributes(html: string, start: number) {
  const attributes = new Map<string, string>();
  let index = start;
  while (index < html.length) {
    while (index < html.length && /[\s/]/.test(html[index])) index++;
    if (html[index] === '>' || index >= html.length) break;
    AttributePattern.lastIndex = index;
    const match = AttributePattern.exec(html);
    if (!match) {
      index++;
      continue;
    }
    const name = match[1].toLowerCase();
    if (!attributes.has(name))
      attributes.set(name, match[2] ?? match[3] ?? match[4] ?? '');
    index = AttributePattern.lastIndex;
  }
  return attributes;
}

function metaTags(html: string) {
  const tags = new Map<string, string>();
  for (const match of html.matchAll(/<meta(?=[\s/>])/gi)) {
    const attributes = readAttributes(html, match.index + match[0].length);
    const key = attributes.get('property') ?? attributes.get('name');
    const content = attributes.get('content');
    if (key && content !== undefined && !tags.has(key.toLowerCase()))
      tags.set(key.toLowerCase(), decodeEntities(content));
  }
  return tags;
}

function absoluteHttpUrl(value: string | undefined, base: string) {
  if (!value) return;
  try {
    const url = new URL(value, base);
    return url.protocol === 'http:' || url.protocol === 'https:'
      ? url.href
      : undefined;
  } catch {
    return;
  }
}

/** Reads Open Graph / Twitter / plain HTML metadata; every field is optional. */
export function parseLinkMetadata(html: string, pageUrl: string) {
  const tags = metaTags(html);
  const pick = (...keys: string[]) =>
    keys.map((key) => tags.get(key)).find(Boolean) || undefined;
  const documentTitle = /<title[^>]*>([\s\S]*?)<\/title>/i.exec(html)?.[1];
  return {
    title:
      pick('og:title', 'twitter:title') ||
      (documentTitle ? decodeEntities(documentTitle) : undefined) ||
      undefined,
    description: pick('og:description', 'twitter:description', 'description'),
    siteName: pick('og:site_name'),
    image: absoluteHttpUrl(
      pick('og:image', 'og:image:url', 'twitter:image', 'twitter:image:src'),
      pageUrl,
    ),
  };
}

function unavailable(url: string): LinkPreview {
  return { url, status: 'unavailable' };
}

export function createLinkPreviewService(deps: LinkPreviewDeps) {
  const cache = new Map<string, LinkPreview>();
  const inflight = new Map<string, Promise<void>>();
  const cacheLimit = deps.cacheLimit ?? DefaultCacheLimit;

  function remember(url: string, preview: LinkPreview) {
    cache.delete(url);
    cache.set(url, preview);
    while (cache.size > cacheLimit) {
      const oldest = cache.keys().next().value;
      if (oldest === undefined) break;
      cache.delete(oldest);
    }
  }

  /** Cached preview, or a loading placeholder that never triggers a network request. */
  function peek(url: string): LinkPreview {
    return cache.get(url) || { url, status: 'loading' };
  }

  async function fetchAndRemember(url: string) {
    let ready: LinkPreview | null = null;
    try {
      const page = await deps.fetchHtml(url);
      const metadata = page
        ? parseLinkMetadata(page.html, page.finalUrl)
        : null;
      if (
        metadata &&
        (metadata.title || metadata.description || metadata.image)
      )
        ready = { url, status: 'ready', ...metadata };
    } catch {
      ready = null;
    }
    remember(url, ready ? { ...ready, image: undefined } : unavailable(url));
    deps.onSettled?.(url);
    if (!(ready?.image && deps.fetchImage)) return;
    const image = await deps.fetchImage(ready.image).catch(() => null);
    if (!image) return;
    remember(url, { ...ready, image });
    deps.onSettled?.(url);
  }

  /** Starts a background fetch once per URL; settled results are cached. */
  function load(url: string) {
    if (cache.has(url)) return;
    if (inflight.has(url)) return;
    const pending = fetchAndRemember(url).finally(() => inflight.delete(url));
    inflight.set(url, pending);
  }

  return { peek, load };
}
