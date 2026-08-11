import assert from 'node:assert/strict';
import test from 'node:test';
import { createBrowserNevermindApi } from './nevermind-api';

type Listener = (event: MessageEvent<string>) => void;
type EventSourceConstructor = new (url: string) => EventSource;

class FakeEventSource {
  listeners = new Map<string, Listener[]>();
  closed = false;

  constructor(readonly url: string) {}

  addEventListener(type: string, listener: Listener) {
    this.listeners.set(type, [...(this.listeners.get(type) || []), listener]);
  }

  close() {
    this.closed = true;
  }

  emit(type: string, payload: unknown) {
    for (const listener of this.listeners.get(type) || []) {
      listener({ data: JSON.stringify(payload) } as MessageEvent<string>);
    }
  }
}

test('routes RPC calls and forwards matching SSE events', async () => {
  const requests: Array<{ method: string; params: unknown }> = [];
  let source: FakeEventSource | undefined;
  const api = createBrowserNevermindApi({
    rpcUrl: 'https://api.example.test/rpc',
    eventUrl: 'https://api.example.test/events',
    token: 'secret',
    eventSource: class extends FakeEventSource {
      constructor(url: string) {
        super(url);
        source = this;
      }
    } as unknown as EventSourceConstructor,
    fetch: async (_input, init) => {
      requests.push(JSON.parse(String(init?.body)));
      return new Response(JSON.stringify({ authed: true }), { status: 200 });
    },
  });

  const authUpdates: Array<{ authed: boolean }> = [];
  const unsubscribe = api.onNevermindAuthChanged((status) =>
    authUpdates.push(status),
  );
  assert.equal(source?.url, 'https://api.example.test/events?token=secret');
  source?.emit('nevermind-auth-changed', { authed: true });
  unsubscribe();
  source?.emit('nevermind-auth-changed', { authed: false });

  assert.deepEqual(await api.getNevermindAuthStatus(), { authed: true });
  assert.deepEqual(requests, [{ method: 'getNevermindAuthStatus' }]);
  assert.deepEqual(authUpdates, [{ authed: true }]);
  api.close();
  assert.equal(source?.closed, true);
});

test('keeps browser-native-only APIs explicit and safe', async () => {
  const api = createBrowserNevermindApi({
    rpcUrl: 'https://api.example.test/rpc',
    eventUrl: 'https://api.example.test/events',
    eventSource: FakeEventSource as unknown as EventSourceConstructor,
    fetch: async () => new Response('{}', { status: 200 }),
  });

  assert.deepEqual(await api.pickFormFieldPaths({}), {
    canceled: true,
    paths: [],
  });
  assert.deepEqual(await api.requestCameraAccess(), {
    ok: false,
    status: 'unavailable',
  });
  assert.throws(() => api.testInvoke(), /unavailable in the browser/);
  api.close();
});
