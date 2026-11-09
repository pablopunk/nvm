// Smoke test of the pi host against a fake backend: one turn that calls a host tool and then answers.
// Run by hand: node test/smoke.mjs (after npm ci). It needs no network and no account. PI_NODE and PI_HOST point it at a
// packaged runtime instead, such as the one inside the app.
import { spawn } from 'node:child_process';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import readline from 'node:readline';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));

function sse(response, chunks) {
  response.writeHead(200, { 'Content-Type': 'text/event-stream' });
  for (const chunk of chunks) response.write(`data: ${JSON.stringify(chunk)}\n\n`);
  response.write('data: [DONE]\n\n');
  response.end();
}

const chunk = (delta, finish = null) => ({
  id: 'c1',
  object: 'chat.completion.chunk',
  created: 1,
  model: 'fake',
  choices: [{ index: 0, delta, finish_reason: finish }],
});

const server = http.createServer((request, response) => {
  if (request.url.startsWith('/api/v1/active-model')) {
    const { port } = server.address();
    response.setHeader('Content-Type', 'application/json');
    return response.end(
      JSON.stringify({
        id: 'fake', name: 'Fake', api: 'openai-completions', provider: 'nevermind',
        baseUrl: `http://127.0.0.1:${port}/v1`, reasoning: false, input: ['text'], contextWindow: 32000, maxTokens: 2000,
      }),
    );
  }
  let body = '';
  request.on('data', (part) => (body += part));
  request.on('end', () => {
    const sawToolResult = body.includes('"role":"tool"');
    if (sawToolResult) {
      return sse(response, [chunk({ role: 'assistant', content: 'The host said ' }), chunk({ content: 'hello back' }), chunk({}, 'stop')]);
    }
    sse(response, [
      chunk({ role: 'assistant', tool_calls: [{ index: 0, id: 'call_1', type: 'function', function: { name: 'echo_host', arguments: '{"text":"hello"}' } }] }),
      chunk({}, 'tool_calls'),
    ]);
  });
});

await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const port = server.address().port;
const child = spawn(process.env.PI_NODE || 'node', [process.env.PI_HOST || path.join(here, '..', 'host.mjs')], { stdio: ['pipe', 'pipe', 'inherit'] });
const events = [];
const lines = readline.createInterface({ input: child.stdout });
const done = new Promise((resolve, reject) => {
  const timer = setTimeout(() => reject(new Error('timed out; events: ' + JSON.stringify(events))), 60000);
  lines.on('line', (line) => {
    const event = JSON.parse(line);
    events.push(event);
    if (event.type === 'ready') {
      child.stdin.write(JSON.stringify({
        type: 'prompt', id: 't1', baseUrl: `http://127.0.0.1:${port}`, token: 'test-token', headers: {}, role: '', system: 'Be brief.',
        text: 'say hello via the tool', images: [], tools: [{ name: 'echo_host', label: 'Echo', description: 'Echoes text back.', parameters: { type: 'object', properties: { text: { type: 'string' } }, required: ['text'], additionalProperties: false } }],
        workspaceDir: path.join(os.tmpdir(), 'pi-host-smoke', 'work'), agentDir: path.join(os.tmpdir(), 'pi-host-smoke', 'agent'),
      }) + '\n');
    }
    if (event.type === 'tool_call') {
      child.stdin.write(JSON.stringify({ type: 'tool_result', callId: event.callId, text: 'echo: ' + event.arguments.text, status: 'ran' }) + '\n');
    }
    if (event.type === 'done' || event.type === 'error' || event.type === 'aborted') {
      clearTimeout(timer);
      resolve(event);
    }
  });
});
try {
  const last = await done;
  const text = events.filter((e) => e.type === 'delta').map((e) => e.text).join('');
  const called = events.some((e) => e.type === 'tool_call' && e.name === 'echo_host');
  console.log(JSON.stringify({ last: last.type, message: last.message, called, text }));
  if (last.type !== 'done' || !called || !text.includes('hello back')) process.exitCode = 1;
} finally {
  child.stdin.write('{"type":"shutdown"}\n');
  server.close();
}
