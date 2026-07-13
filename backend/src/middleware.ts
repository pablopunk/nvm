import { defineMiddleware } from 'astro:middleware';
import { waitUntil } from '@vercel/functions';
import { flushLogs } from './lib/log';

// Axiom batches events in memory. Keep the function alive after the response so
// each request's logs are delivered without adding latency to the response.
export const onRequest = defineMiddleware(async (_context, next) => {
  try {
    return await next();
  } finally {
    waitUntil(flushLogs());
  }
});
