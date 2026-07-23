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
};

export async function createDesignTokenStudioServer(
  options: DesignTokenStudioServerOptions,
) {
  const token = crypto.randomBytes(32).toString('hex');
  const server = http.createServer(async (request, response) => {
    response.setHeader('access-control-allow-origin', options.allowedOrigin);
    response.setHeader('access-control-allow-headers', 'content-type, x-nvm-token');
    response.setHeader('access-control-allow-methods', 'GET, PUT, DELETE, OPTIONS');
    response.setHeader('cache-control', 'no-store');
    if (request.method === 'OPTIONS') {
      response.statusCode = 204;
      response.end();
      return;
    }
    if (
      request.headers.origin !== options.allowedOrigin ||
      request.headers['x-nvm-token'] !== token
    ) {
      response.statusCode = 403;
      response.end('Forbidden');
      return;
    }
    try {
      const state = await handleRequest(request, options);
      response.setHeader('content-type', 'application/json');
      response.end(JSON.stringify(state));
    } catch (error) {
      response.statusCode = 400;
      response.setHeader('content-type', 'text/plain');
      response.end(error instanceof Error ? error.message : 'Invalid request');
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const { port } = server.address() as AddressInfo;
  return {
    apiUrl: `http://127.0.0.1:${port}/design-tokens`,
    token,
    close: () => new Promise<void>((resolve) => server.close(() => resolve())),
  };
}

async function handleRequest(
  request: http.IncomingMessage,
  options: DesignTokenStudioServerOptions,
) {
  if (request.method === 'GET') return options.getState();
  if (request.method === 'DELETE') return options.resetState();
  if (request.method === 'PUT') {
    const body = await readJsonBody(request);
    return options.setState(body as DesignTokenOverrides);
  }
  throw new Error('Method not allowed');
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
