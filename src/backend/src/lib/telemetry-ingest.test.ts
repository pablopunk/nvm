import assert from 'node:assert/strict';
import test from 'node:test';
import { gzipSync } from 'node:zlib';
import {
  countByEvent,
  forwardToPosthog,
  MAX_BATCH_EVENTS,
  readIngestBody,
  sanitizeBatch,
} from './telemetry-ingest';

const NOW = Date.parse('2026-10-30T12:00:00.000Z');
const INSTALL = '3f2b8c1e-9d4a-4e6b-8a1c-5d7e9f0a1b2c';

function event(overrides: Record<string, unknown> = {}) {
  return {
    event: 'feature.used',
    distinct_id: INSTALL,
    uuid: '0198a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2b',
    timestamp: '2026-10-30T11:59:00.000Z',
    properties: { feature: 'command.run', variant: 'app', app_version: '1.0.0' },
    ...overrides,
  };
}

function request(body: BodyInit, headers: Record<string, string> = {}) {
  return new Request('https://api.nvm.fyi/api/ingest/batch/', {
    method: 'POST',
    body,
    headers,
  });
}

test('only known events from a real install id are accepted', () => {
  const result = sanitizeBatch(
    {
      batch: [
        event(),
        event({ event: 'something.else' }),
        event({ distinct_id: 'user@example.com' }),
        event({ properties: 'nope' }),
        'garbage',
      ],
    },
    NOW,
  );
  assert.equal(result?.events.length, 1);
  assert.equal(result?.dropped, 4);
  assert.equal(sanitizeBatch({ nothing: true }, NOW), null);
  assert.equal(sanitizeBatch('x', NOW), null);
});

test('clients cannot set person data, location, ip or the project key', () => {
  const result = sanitizeBatch(
    {
      batch: [
        event({
          properties: {
            feature: 'command.run',
            $set: { email: 'a@b.c' },
            $set_once: { x: 1 },
            $ip: '1.2.3.4',
            $process_person_profile: true,
            $geoip_disable: false,
            api_key: 'phc_x',
            'bad key!': 1,
          },
        }),
      ],
    },
    NOW,
  );
  const properties = result?.events[0]?.properties ?? {};
  assert.deepEqual(Object.keys(properties).sort(), [
    '$geoip_disable',
    '$process_person_profile',
    'feature',
  ]);
  assert.equal(properties.$process_person_profile, false);
  assert.equal(properties.$geoip_disable, true);
});

test('owner events require a hashed account id', () => {
  const owner = (ownerId: unknown) =>
    event({
      event: 'install.owner_observed',
      properties: { owner_source: 'account', owner_id: ownerId },
    });
  const result = sanitizeBatch(
    { batch: [owner('a'.repeat(64)), owner('user@example.com'), owner(7)] },
    NOW,
  );
  assert.equal(result?.events.length, 1);
});

test('timestamps outside the accepted window fall back to server time', () => {
  const result = sanitizeBatch(
    {
      batch: [
        event({ timestamp: '2020-01-01T00:00:00.000Z' }),
        event({ timestamp: '2030-01-01T00:00:00.000Z' }),
        event({ timestamp: 'not a date' }),
        event(),
      ],
    },
    NOW,
  );
  assert.deepEqual(
    result?.events.map((item) => item.timestamp),
    [undefined, undefined, undefined, '2026-10-30T11:59:00.000Z'],
  );
});

test('a batch is capped and oversized events are dropped', () => {
  const many = Array.from({ length: MAX_BATCH_EVENTS + 25 }, () => event());
  assert.equal(sanitizeBatch({ batch: many }, NOW)?.events.length, MAX_BATCH_EVENTS);
  const huge = event({ properties: { blob: 'x'.repeat(80 * 1024) } });
  assert.equal(sanitizeBatch({ batch: [huge] }, NOW)?.events.length, 0);
});

test('an oversized exception is trimmed, not dropped', () => {
  const frame = { filename: 'app:///main.js', lineno: 1, colno: 1, function: 'f' };
  const exception = event({
    event: '$exception',
    properties: {
      $exception_list: [
        {
          type: 'Error',
          value: 'm'.repeat(200 * 1024),
          stacktrace: { frames: Array.from({ length: 500 }, () => frame) },
        },
      ],
    },
  });
  const result = sanitizeBatch({ batch: [exception] }, NOW);
  assert.equal(result?.events.length, 1);
  const entry = (result?.events[0]?.properties.$exception_list as Array<{
    value: string;
    stacktrace: { frames: unknown[] };
  }>)[0];
  assert.equal(entry?.value.length, 2_000);
  assert.equal(entry?.stacktrace.frames.length, 50);
});

test('a body past the cap is refused while streaming, even without a length header', async () => {
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      for (let index = 0; index < 20; index += 1) {
        controller.enqueue(new Uint8Array(32 * 1024));
      }
      controller.close();
    },
  });
  const streamed = new Request('https://api.nvm.fyi/api/ingest/batch/', {
    method: 'POST',
    body: stream,
    duplex: 'half',
  } as RequestInit);
  assert.deepEqual(await readIngestBody(streamed), { ok: false, status: 413 });
});

test('events are counted by name for the archive log', () => {
  const result = sanitizeBatch(
    { batch: [event(), event(), event({ event: 'app.launched' })] },
    NOW,
  );
  assert.deepEqual(countByEvent(result?.events ?? []), {
    'feature.used': 2,
    'app.launched': 1,
  });
});

test('plain and gzip bodies decode; oversized or malformed bodies are refused', async () => {
  const payload = JSON.stringify({ batch: [event()] });
  assert.deepEqual(await readIngestBody(request(payload)), {
    ok: true,
    value: JSON.parse(payload),
  });
  assert.deepEqual(
    await readIngestBody(
      request(gzipSync(payload), { 'content-encoding': 'gzip' }),
    ),
    { ok: true, value: JSON.parse(payload) },
  );
  assert.deepEqual(await readIngestBody(request('{ not json')), {
    ok: false,
    status: 400,
  });
  assert.deepEqual(
    await readIngestBody(
      request(gzipSync('x'.repeat(2 * 1024 * 1024)), {
        'content-encoding': 'gzip',
      }),
    ),
    { ok: false, status: 413 },
  );
  assert.deepEqual(
    await readIngestBody(request('x'.repeat(300 * 1024))),
    { ok: false, status: 413 },
  );
});

test('forwarding uses the server-held key and reports delivery failure', async () => {
  const calls: Array<{ url: string; body: { api_key: string; batch: unknown[] } }> = [];
  const fetchImpl = (async (url: string, init: RequestInit) => {
    calls.push({ url, body: JSON.parse(String(init.body)) });
    return new Response('{}', { status: calls.length === 1 ? 200 : 503 });
  }) as unknown as typeof fetch;
  const events = sanitizeBatch({ batch: [event()] }, NOW)!.events;
  assert.equal(
    await forwardToPosthog(events, {
      key: 'phc_server',
      host: 'https://eu.i.posthog.com/',
      fetchImpl,
    }),
    true,
  );
  assert.equal(
    await forwardToPosthog(events, {
      key: 'phc_server',
      host: 'https://eu.i.posthog.com',
      fetchImpl,
    }),
    false,
  );
  assert.equal(calls[0]?.url, 'https://eu.i.posthog.com/batch/');
  assert.equal(calls[0]?.body.api_key, 'phc_server');
  assert.equal(calls[0]?.body.batch.length, 1);
});
