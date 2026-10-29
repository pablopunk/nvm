import assert from 'node:assert/strict';
import test from 'node:test';
import { registerInstanceEvents } from './instance-events';

test('a deep link that cannot run during maintenance remains buffered', () => {
  const handlers = new Map<string, (...args: any[]) => void>();
  const buffered: string[] = [];
  registerInstanceEvents({
    app: {
      on: (event, callback) => {
        handlers.set(event, callback);
      },
    },
    scheme: 'nvm',
    testMode: false,
    isReady: () => true,
    showPalette: () => {},
    processLink: () => {
      throw new Error('maintenance');
    },
    bufferedLinks: buffered,
  });
  handlers.get('open-url')?.({}, 'nvm://auth?code=123');
  assert.deepEqual(buffered, ['nvm://auth?code=123']);
});
