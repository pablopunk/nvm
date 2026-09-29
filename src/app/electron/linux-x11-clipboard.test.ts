import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';
import test from 'node:test';
import type { ChildProcess, spawn } from 'node:child_process';
import { createLinuxX11Clipboard } from './linux-x11-clipboard';

test('writes only the X11 Clipboard, not the Primary selection', async () => {
  const calls: string[][] = [];
  const clipboard = createLinuxX11Clipboard({
    environment: { DISPLAY: ':99', XDG_SESSION_TYPE: 'x11' },
    findCommand: () => '/usr/bin/xclip',
    spawnCommand: ((_command: string, args: string[]) => {
      calls.push(args);
      const child = new EventEmitter() as ChildProcess;
      child.stdin = {
        end(text: string) {
          assert.equal(text, 'probe');
          queueMicrotask(() => child.emit('close', 0));
        },
        on() {
          return this;
        },
      } as unknown as ChildProcess['stdin'];
      return child;
    }) as typeof spawn,
  });
  assert.equal(clipboard.available(), true);
  await clipboard.writeText('probe');
  assert.deepEqual(calls, [['-selection', 'clipboard', '-in']]);
});

test('reports a missing X11 Clipboard writer as unavailable', async () => {
  const clipboard = createLinuxX11Clipboard({
    environment: { DISPLAY: ':99' },
    findCommand: () => null,
  });
  assert.equal(clipboard.available(), false);
  await assert.rejects(clipboard.writeText('probe'), /Install xclip/);
});

test('rejects rich X11 clipboard data before it can clear the source selection', () => {
  const clipboard = createLinuxX11Clipboard({
    environment: { DISPLAY: ':99' },
  });
  assert.doesNotThrow(() => clipboard.validateSnapshot({}));
  assert.throws(
    () => clipboard.validateSnapshot({ html: '<p>hello</p>' }),
    /Cannot preserve a formatted X11 clipboard/,
  );
});
