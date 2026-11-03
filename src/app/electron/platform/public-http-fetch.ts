import dns from 'node:dns';
import http from 'node:http';
import https from 'node:https';
import net from 'node:net';

export type PublicHttpResponse = {
  status: number;
  contentType: string;
  body: Buffer;
  finalUrl: string;
};

export type PublicHttpFetchOptions = {
  accept: string;
  maxBytes: number;
  timeoutMs: number;
  maxRedirects?: number;
  /** Stops reading early once the accumulated text satisfies this check. */
  isComplete?: (text: string) => boolean;
  /** Lets tests substitute name resolution. */
  lookup?: typeof dns.lookup;
  /** Lets tests widen the address policy; production always uses `isPublicAddress`. */
  addressPolicy?: (address: string) => boolean;
};

const DefaultMaxRedirects = 4;

/** Loopback, private, link-local, CGNAT, multicast, documentation, and reserved ranges. */
const NonPublicAddresses = (() => {
  const list = new net.BlockList();
  const v4: Array<[string, number]> = [
    ['0.0.0.0', 8],
    ['10.0.0.0', 8],
    ['100.64.0.0', 10],
    ['127.0.0.0', 8],
    ['169.254.0.0', 16],
    ['172.16.0.0', 12],
    ['192.0.0.0', 24],
    ['192.0.2.0', 24],
    ['192.168.0.0', 16],
    ['198.18.0.0', 15],
    ['198.51.100.0', 24],
    ['203.0.113.0', 24],
    ['224.0.0.0', 4],
    ['240.0.0.0', 4],
  ];
  for (const [network, prefix] of v4) list.addSubnet(network, prefix, 'ipv4');
  const v6: Array<[string, number]> = [
    ['::', 128],
    ['::1', 128],
    ['64:ff9b::', 96],
    ['100::', 64],
    ['2001:db8::', 32],
    ['fc00::', 7],
    ['fe80::', 10],
    ['ff00::', 8],
  ];
  for (const [network, prefix] of v6) list.addSubnet(network, prefix, 'ipv6');
  return list;
})();

const MappedIpv4 = /^::ffff:(\d+\.\d+\.\d+\.\d+)$/i;

export function isPublicAddress(address: string) {
  const family = net.isIP(address);
  if (!family) return false;
  const mapped = MappedIpv4.exec(address)?.[1];
  if (mapped) return isPublicAddress(mapped);
  return !NonPublicAddresses.check(address, family === 4 ? 'ipv4' : 'ipv6');
}

function unbracketed(hostname: string) {
  return hostname.replace(/^\[|\]$/g, '');
}

/**
 * Resolves the host once, rejects it unless every address is public, and hands
 * that exact address to the socket so a second DNS answer cannot rebind it.
 */
function pinnedPublicLookup(
  resolve: typeof dns.lookup,
  isAllowed: (address: string) => boolean,
): typeof dns.lookup {
  return ((hostname: string, socketOptions: any, callback: any) => {
    resolve(hostname, { all: true, verbatim: true }, (error, addresses) => {
      if (error) return callback(error);
      const list = addresses as dns.LookupAddress[];
      if (!(list.length && list.every((a) => isAllowed(a.address))))
        return callback(new Error('Host does not resolve to a public address'));
      if (socketOptions?.all) return callback(null, list);
      callback(null, list[0].address, list[0].family);
    });
  }) as typeof dns.lookup;
}

function requestOnce(
  url: URL,
  options: PublicHttpFetchOptions,
): Promise<{
  response: PublicHttpResponse;
  redirect?: string;
}> {
  return new Promise((resolve, reject) => {
    if (url.protocol !== 'http:' && url.protocol !== 'https:')
      return reject(new Error('Unsupported protocol'));
    const isAllowed = options.addressPolicy ?? isPublicAddress;
    const host = unbracketed(url.hostname);
    if (net.isIP(host) && !isAllowed(host))
      return reject(new Error('Non-public address'));
    const client = url.protocol === 'https:' ? https : http;
    const request = client.request(
      url,
      {
        method: 'GET',
        lookup: pinnedPublicLookup(options.lookup ?? dns.lookup, isAllowed),
        signal: AbortSignal.timeout(options.timeoutMs),
        headers: {
          accept: options.accept,
          'user-agent': 'Mozilla/5.0 (compatible; Nevermind link preview)',
        },
      },
      (incoming) => {
        const status = incoming.statusCode || 0;
        const contentType = String(incoming.headers['content-type'] || '');
        const location = incoming.headers.location;
        if (status >= 300 && status < 400 && location) {
          incoming.resume();
          let next: string;
          try {
            next = new URL(location, url).href;
          } catch {
            return reject(new Error('Invalid redirect'));
          }
          return resolve({
            response: {
              status,
              contentType,
              body: Buffer.alloc(0),
              finalUrl: url.href,
            },
            redirect: next,
          });
        }
        const chunks: Buffer[] = [];
        const decoder = new TextDecoder();
        let received = 0;
        let text = '';
        function finish() {
          incoming.destroy();
          resolve({
            response: {
              status,
              contentType,
              body: Buffer.concat(chunks),
              finalUrl: url.href,
            },
          });
        }
        incoming.on('data', (chunk: Buffer) => {
          received += chunk.byteLength;
          chunks.push(chunk);
          if (options.isComplete) {
            text += decoder.decode(chunk, { stream: true });
            if (options.isComplete(text)) return finish();
          }
          if (received > options.maxBytes) {
            if (options.isComplete) return finish();
            incoming.destroy();
            reject(new Error('Response too large'));
          }
        });
        incoming.on('end', finish);
        incoming.on('error', reject);
      },
    );
    request.on('error', reject);
    request.end();
  });
}

/**
 * GET that only ever talks to public internet addresses: each redirect hop is
 * re-validated, DNS answers are checked and pinned, and the body is bounded.
 */
export async function fetchPublicHttp(
  startUrl: string,
  options: PublicHttpFetchOptions,
): Promise<PublicHttpResponse> {
  const deadline = Date.now() + options.timeoutMs;
  let url = new URL(startUrl);
  const maxRedirects = options.maxRedirects ?? DefaultMaxRedirects;
  for (let hop = 0; hop <= maxRedirects; hop++) {
    const remaining = deadline - Date.now();
    if (remaining <= 0) throw new Error('Request timed out');
    const { response, redirect } = await requestOnce(url, {
      ...options,
      timeoutMs: remaining,
    });
    if (!redirect) return response;
    url = new URL(redirect);
  }
  throw new Error('Too many redirects');
}
