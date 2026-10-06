const http = require('node:http');
const zlib = require('node:zlib');

async function createDiagnosticSmokeCollector() {
  const events = [];
  const failures = [];
  const server = http.createServer(function receiveEnvelope(request, response) {
    const chunks = [];
    let size = 0;
    request.on('data', function collectChunk(chunk) {
      size += chunk.length;
      if (size > 1024 * 1024) request.destroy();
      else chunks.push(chunk);
    });
    request.on('end', function readEnvelope() {
      try {
        const data = Buffer.concat(chunks);
        const text = (
          request.headers['content-encoding'] === 'gzip'
            ? zlib.gunzipSync(data)
            : data
        ).toString();
        if (text.includes('PRIVATE_'))
          throw new Error(
            'Private fixture content reached the diagnostic transport',
          );
        const lines = text.trim().split('\n');
        for (let index = 1; index < lines.length; index += 2) {
          const header = JSON.parse(lines[index]);
          if (!['event', 'transaction'].includes(header.type))
            throw new Error(`Unexpected diagnostic item: ${header.type}`);
          events.push({
            type: header.type,
            event: JSON.parse(lines[index + 1]),
          });
        }
        response.writeHead(200, { 'Content-Type': 'application/json' });
        response.end('{}');
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
    dsn: `http://qa@127.0.0.1:${server.address().port}/1`,
    close: function closeCollector() {
      return new Promise(function close(resolve) {
        server.close(resolve);
      });
    },
  };
}

module.exports = { createDiagnosticSmokeCollector };
