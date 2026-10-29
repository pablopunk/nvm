const http = require('node:http');
const zlib = require('node:zlib');

// Loopback stand-in for the PostHog ingestion host. posthog-node posts
// `{ api_key, batch: [{ event, distinct_id, properties }] }` to `/batch/`.
async function createDiagnosticSmokeCollector() {
  const events = [];
  const failures = [];
  const server = http.createServer(function receiveBatch(request, response) {
    const chunks = [];
    let size = 0;
    request.on('data', function collectChunk(chunk) {
      size += chunk.length;
      if (size > 1024 * 1024) request.destroy();
      else chunks.push(chunk);
    });
    request.on('end', function readBatch() {
      try {
        const data = Buffer.concat(chunks);
        const text = (
          request.headers['content-encoding'] === 'gzip'
            ? zlib.gunzipSync(data)
            : data
        ).toString();
        const body = JSON.parse(text);
        for (const item of body.batch ?? [body]) {
          if (typeof item.event !== 'string')
            throw new Error('Unexpected diagnostic payload');
          events.push({
            event: item.event,
            distinctId: item.distinct_id,
            properties: item.properties ?? {},
          });
        }
        response.writeHead(200, { 'Content-Type': 'application/json' });
        response.end('{"status":1}');
      } catch (error) {
        failures.push(String(error));
        response.writeHead(400);
        response.end();
      }
    });
  });
  await new Promise(function listen(resolve) {
    server.listen(0, '127.0.0.1', resolve);
  });
  return {
    events,
    failures,
    host: `http://127.0.0.1:${server.address().port}`,
    close: function closeCollector() {
      return new Promise(function close(resolve) {
        server.close(resolve);
      });
    },
  };
}

module.exports = { createDiagnosticSmokeCollector };
