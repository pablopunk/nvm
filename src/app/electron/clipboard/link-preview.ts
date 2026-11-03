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
  /** Notified when a background load settles so open views can refresh in place. */
  onSettled?: (url: string) => void;
  cacheLimit?: number;
};

const DefaultCacheLimit = 200;
const PrivateIpv4 =
  /^(10\.|127\.|169\.254\.|192\.168\.|172\.(1[6-9]|2\d|3[01])\.|0\.)/;

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
  if (host === 'localhost' || host.endsWith('.local') || host.includes(':'))
    return null;
  if (PrivateIpv4.test(host) || !host.includes('.')) return null;
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

function metaTags(html: string) {
  const tags = new Map<string, string>();
  for (const tag of html.match(/<meta\b[^>]*>/gi) || []) {
    const key = /(?:property|name)\s*=\s*["']([^"']+)["']/i.exec(tag)?.[1];
    const content = /content\s*=\s*(?:"([^"]*)"|'([^']*)')/i.exec(tag);
    const value = content?.[1] ?? content?.[2];
    if (key && value !== undefined && !tags.has(key.toLowerCase()))
      tags.set(key.toLowerCase(), decodeEntities(value));
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
    try {
      const page = await deps.fetchHtml(url);
      const metadata = page
        ? parseLinkMetadata(page.html, page.finalUrl)
        : null;
      const hasMetadata = Boolean(
        metadata && (metadata.title || metadata.description || metadata.image),
      );
      remember(
        url,
        metadata && hasMetadata
          ? { url, status: 'ready', ...metadata }
          : { url, status: 'unavailable' },
      );
    } catch {
      remember(url, { url, status: 'unavailable' });
    }
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

const MaxHtmlBytes = 256 * 1024;
const FetchTimeoutMs = 6000;

/** Production fetch port: bounded, HTML-only, with a short timeout. */
export async function fetchHtmlHead(
  url: string,
  fetchImpl: typeof fetch = fetch,
) {
  const response = await fetchImpl(url, {
    redirect: 'follow',
    signal: AbortSignal.timeout(FetchTimeoutMs),
    headers: {
      accept: 'text/html,application/xhtml+xml',
      'user-agent': 'Mozilla/5.0 (compatible; Nevermind link preview)',
    },
  });
  const contentType = response.headers.get('content-type') || '';
  if (!(response.ok && /html/i.test(contentType) && response.body)) {
    await response.body?.cancel();
    return null;
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let html = '';
  let received = 0;
  while (received < MaxHtmlBytes) {
    const { done, value } = await reader.read();
    if (done) break;
    received += value.byteLength;
    html += decoder.decode(value, { stream: true });
    if (/<\/head>/i.test(html)) break;
  }
  await reader.cancel();
  return { html, finalUrl: response.url || url };
}
