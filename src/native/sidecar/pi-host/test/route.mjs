// Route test of the pi host against a fake backend: prepare readies the session without a prompt, every request sends
// its prompt size as `chars` next to the `model` role, and a route the backend blocks is refused with its own code.
// Run by hand: node test/route.mjs (after npm ci). It needs no network and no account.
import { spawn } from 'node:child_process';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import readline from 'node:readline';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const requests = [];
let blocked = false;

const server = http.createServer((request, response) => {
  if (!request.url.startsWith('/api/v1/active-model')) {
    response.writeHead(404);
    return response.end();
  }
  requests.push(new URL(request.url, 'http://x').searchParams);
  const { port } = server.address();
  response.setHeader('Content-Type', 'application/json');
  response.end(
    JSON.stringify({
      id: 'fake', name: 'Fake', api: 'openai-completions', provider: 'nevermind',
      baseUrl: `http://127.0.0.1:${port}/v1`, reasoning: false, input: ['text'], contextWindow: 32000, maxTokens: 2000,
      ...(blocked ? { notice: 'blocked', noticeReason: 'free_spend_cap_reached' } : {}),
    }),
  );
});
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const port = server.address().port;
const child = spawn(process.env.PI_NODE || 'node', [process.env.PI_HOST || path.join(here, '..', 'host.mjs')], { stdio: ['pipe', 'pipe', 'inherit'] });
const lines = readline.createInterface({ input: child.stdout });
const waiting = [];
lines.on('line', (line) => waiting.shift()?.(JSON.parse(line)));
const next = () => new Promise((resolve) => waiting.push(resolve));
const request = (type, id, chars) => ({
  type, id, baseUrl: `http://127.0.0.1:${port}`, token: 'test-token', headers: {}, role: 'fast', system: 'Be brief.', chars,
  text: '', images: [], tools: [],
  workspaceDir: path.join(os.tmpdir(), 'pi-host-route', 'work'), agentDir: path.join(os.tmpdir(), 'pi-host-route', 'agent'),
});
const failures = [];
const expect = (ok, what) => ok || failures.push(what);

try {
  expect((await next()).type === 'ready', 'host says ready');
  child.stdin.write(JSON.stringify(request('prepare', 'p1', 123)) + '\n');
  const prepared = await next();
  expect(prepared.type === 'prepared', `prepare answers prepared, got ${JSON.stringify(prepared)}`);
  expect(requests.length === 1 && requests[0].get('model') === 'fast' && requests[0].get('chars') === '123', 'prepare asks for the role and size');

  blocked = true;
  child.stdin.write(JSON.stringify(request('prepare', 'p2', 456)) + '\n');
  const refused = await next();
  expect(refused.type === 'error' && refused.code === 'free_spend_cap_reached', `a blocked route is refused, got ${JSON.stringify(refused)}`);
  expect(requests.at(-1).get('chars') === '456', 'the checked route carries the new size');
} finally {
  child.stdin.write('{"type":"shutdown"}\n');
  server.close();
}
console.log(JSON.stringify({ failures }));
if (failures.length > 0) process.exitCode = 1;
