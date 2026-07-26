import crypto from 'node:crypto';
import http from 'node:http';
import type { AddressInfo } from 'node:net';
import type { DesignTokenOverrides } from '../design-tokens';
import type { DesignTokenState } from '../preload-api';

const MAX_BODY_BYTES = 64 * 1024;

type DesignTokenStudioServerOptions = {
  allowedOrigin: string;
  getState: () => DesignTokenState;
  setState: (overrides: DesignTokenOverrides) => DesignTokenState;
  resetState: () => DesignTokenState;
  rpc?: (method: string, params: unknown) => unknown | Promise<unknown>;
};

type StudioEventClient = http.ServerResponse<http.IncomingMessage>;

export async function createDesignTokenStudioServer(
  options: DesignTokenStudioServerOptions,
) {
  const token = crypto.randomBytes(32).toString('hex');
  const eventClients = new Set<StudioEventClient>();
  const server = http.createServer(async (request, response) => {
    setCorsHeaders(response, options.allowedOrigin);
    if (request.method === 'OPTIONS') {
      response.statusCode = 204;
      response.end();
      return;
    }
    if (!requestIsAuthorized(request, options.allowedOrigin, token)) {
      response.statusCode = 403;
      response.end('Forbidden');
      return;
    }
    try {
      if (request.url?.startsWith('/events')) {
        openEventStream(request, response, eventClients);
        return;
      }
      const result = await handleRequest(request, options);
      response.setHeader('content-type', 'application/json');
      response.end(JSON.stringify(result));
    } catch (error) {
      response.statusCode = error instanceof UnknownRpcMethodError ? 404 : 400;
      response.setHeader('content-type', 'text/plain');
      response.end(error instanceof Error ? error.message : 'Invalid request');
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const { port } = server.address() as AddressInfo;
  const origin = `http://127.0.0.1:${port}`;
  return {
    apiUrl: `${origin}/design-tokens`,
    rpcUrl: `${origin}/rpc`,
    eventUrl: `${origin}/events`,
    token,
    publish(event: string, payload: unknown) {
      const message = `event: ${event}\ndata: ${JSON.stringify(payload)}\n\n`;
      for (const client of eventClients) client.write(message);
    },
    close: () =>
      new Promise<void>((resolve) => {
        for (const client of eventClients) client.end();
        eventClients.clear();
        server.close(() => resolve());
      }),
  };
}

function setCorsHeaders(response: StudioEventClient, allowedOrigin: string) {
  response.setHeader('access-control-allow-origin', allowedOrigin);
  response.setHeader(
    'access-control-allow-headers',
    'authorization, content-type, x-nvm-token',
  );
  response.setHeader(
    'access-control-allow-methods',
    'GET, PUT, DELETE, POST, OPTIONS',
  );
  response.setHeader('cache-control', 'no-store');
}

function requestIsAuthorized(
  request: http.IncomingMessage,
  allowedOrigin: string,
  token: string,
) {
  const requestUrl = new URL(request.url || '/', 'http://127.0.0.1');
  const bearer = String(request.headers.authorization || '').replace(
    /^Bearer\s+/i,
    '',
  );
  return (
    request.headers.origin === allowedOrigin &&
    (request.headers['x-nvm-token'] === token ||
      bearer === token ||
      requestUrl.searchParams.get('token') === token)
  );
}

function openEventStream(
  request: http.IncomingMessage,
  response: StudioEventClient,
  clients: Set<StudioEventClient>,
) {
  response.statusCode = 200;
  response.setHeader('content-type', 'text/event-stream');
  response.setHeader('connection', 'keep-alive');
  response.flushHeaders();
  response.write(': connected\n\n');
  clients.add(response);
  request.on('close', () => clients.delete(response));
}

async function handleRequest(
  request: http.IncomingMessage,
  options: DesignTokenStudioServerOptions,
) {
  const pathname = new URL(request.url || '/', 'http://127.0.0.1').pathname;
  if (pathname === '/design-tokens') {
    if (request.method === 'GET') return options.getState();
    if (request.method === 'DELETE') return options.resetState();
    if (request.method === 'PUT') {
      const body = await readJsonBody(request);
      return options.setState(body as DesignTokenOverrides);
    }
  }
  if (pathname === '/rpc' && request.method === 'POST' && options.rpc) {
    const body = (await readJsonBody(request)) as {
      method?: unknown;
      params?: unknown;
    };
    if (typeof body.method !== 'string') throw new Error('Invalid RPC method');
    const result = await options.rpc(body.method, body.params);
    if (result === undefined) return null;
    return result;
  }
  throw new UnknownRpcMethodError();
}

class UnknownRpcMethodError extends Error {
  constructor() {
    super('Method not allowed');
  }
}

async function readJsonBody(request: http.IncomingMessage) {
  let body = '';
  for await (const chunk of request) {
    body += chunk;
    if (Buffer.byteLength(body) > MAX_BODY_BYTES)
      throw new Error('Request too large');
  }
  return JSON.parse(body || '{}');
}
