import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  type CameraCaptureState,
  cameraErrorMessage,
  cameraVideoConstraints,
  createCameraCapture,
  nextCameraDeviceId,
} from './camera-capture';

function fakeTrack(deviceId: string) {
  const endListeners: Array<() => void> = [];
  return {
    stops: 0,
    getSettings: () => ({ deviceId }),
    label: `Camera ${deviceId}`,
    addEventListener(type: string, listener: () => void) {
      if (type === 'ended') endListeners.push(listener);
    },
    stop() {
      this.stops += 1;
    },
    end() {
      for (const listener of endListeners) listener();
    },
  };
}

function fakeStream(...deviceIds: string[]) {
  const tracks = deviceIds.map(fakeTrack);
  const stream = {
    getVideoTracks: () => tracks,
    getTracks: () => tracks,
  } as unknown as MediaStream;
  return { stream, tracks };
}

function liveDeviceIds(states: CameraCaptureState[]) {
  return states.flatMap((state) =>
    state.status === 'live' ? [state.deviceId] : [],
  );
}

function domError(name: string) {
  return Object.assign(new Error(name), { name });
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function harness(
  overrides: {
    requestAccess?: () => Promise<void>;
    getUserMedia?: (
      constraints: MediaStreamConstraints,
    ) => Promise<MediaStream>;
  } = {},
) {
  const states: CameraCaptureState[] = [];
  const forgotten: number[] = [];
  const capture = createCameraCapture({
    requestAccess: overrides.requestAccess ?? (async () => {}),
    getUserMedia:
      overrides.getUserMedia ?? (async () => fakeStream('default').stream),
    forgetSavedDevice: () => forgotten.push(1),
    onState: (state) => states.push(state),
  });
  return { capture, states, forgotten };
}

test('stopping while a request is pending discards the stream that resolves later', async () => {
  const pending = deferred<MediaStream>();
  const { stream, tracks } = fakeStream('a');
  const { capture, states } = harness({ getUserMedia: () => pending.promise });

  const started = capture.start({ deviceId: 'a', source: 'explicit' });
  await Promise.resolve();
  capture.stop();
  pending.resolve(stream);
  await started;

  assert.equal(tracks[0]?.stops, 1);
  assert.deepEqual(
    states.map((state) => state.status),
    ['starting', 'idle'],
  );
});

test('stopping during permission prevents a later camera request', async () => {
  const permission = deferred<void>();
  let requested = false;
  const { capture } = harness({
    requestAccess: () => permission.promise,
    getUserMedia: async () => {
      requested = true;
      return fakeStream('a').stream;
    },
  });
  const started = capture.start({ source: 'default' });
  capture.stop();
  permission.resolve();
  await started;
  assert.equal(requested, false);
});

test('restarting releases the previous stream before the next one goes live', async () => {
  const first = fakeStream('a');
  const second = fakeStream('b');
  const queue = [first.stream, second.stream];
  const { capture, states } = harness({
    getUserMedia: async () => queue.shift() as MediaStream,
  });

  await capture.start({ deviceId: 'a', source: 'explicit' });
  await capture.start({ deviceId: 'b', source: 'explicit' });

  assert.equal(first.tracks[0]?.stops, 1);
  assert.equal(second.tracks[0]?.stops, 0);
  assert.deepEqual(liveDeviceIds(states), ['a', 'b']);
});

test('a stale saved device falls back to the default camera and forgets the saved id', async () => {
  const fallback = fakeStream('default');
  const exactRequests: boolean[] = [];
  const { capture, states, forgotten } = harness({
    getUserMedia: async (constraints) => {
      const video = constraints.video as MediaTrackConstraints;
      exactRequests.push(video.deviceId !== undefined);
      if (video.deviceId !== undefined) throw domError('NotFoundError');
      return fallback.stream;
    },
  });

  await capture.start({ deviceId: 'gone', source: 'saved' });

  assert.equal(forgotten.length, 1);
  assert.deepEqual(exactRequests, [true, false]);
  assert.deepEqual(liveDeviceIds(states), ['default']);
});

test('an explicit device never falls back or forgets the saved selection', async () => {
  let calls = 0;
  const { capture, states, forgotten } = harness({
    getUserMedia: async () => {
      calls += 1;
      throw domError('OverconstrainedError');
    },
  });

  await capture.start({ deviceId: 'pinned', source: 'explicit' });

  assert.equal(calls, 1);
  assert.equal(forgotten.length, 0);
  assert.equal(states.at(-1)?.status, 'error');
});

test('a track ending on the live stream reports disconnection and releases it once', async () => {
  const live = fakeStream('a');
  const { capture, states } = harness({
    getUserMedia: async () => live.stream,
  });
  await capture.start({ deviceId: 'a', source: 'explicit' });

  live.tracks[0]?.end();
  live.tracks[0]?.end();

  assert.equal(live.tracks[0]?.stops, 1);
  const errors = states.filter((state) => state.status === 'error');
  assert.equal(errors.length, 1);
});

test('playback failure only tears down the stream that failed', async () => {
  const first = fakeStream('a');
  const second = fakeStream('b');
  const queue = [first.stream, second.stream];
  const { capture, states } = harness({
    getUserMedia: async () => queue.shift() as MediaStream,
  });
  await capture.start({ deviceId: 'a', source: 'explicit' });
  await capture.start({ deviceId: 'b', source: 'explicit' });

  capture.playbackFailed(first.stream);
  assert.notEqual(states.at(-1)?.status, 'error');
  assert.equal(second.tracks[0]?.stops, 0);

  capture.playbackFailed(second.stream);
  assert.equal(states.at(-1)?.status, 'error');
  assert.equal(second.tracks[0]?.stops, 1);
});

test('denied system access stops before any device is requested', async () => {
  let requested = false;
  const { capture, states } = harness({
    requestAccess: async () => {
      throw new Error('Camera access is denied in system privacy settings.');
    },
    getUserMedia: async () => {
      requested = true;
      return fakeStream('a').stream;
    },
  });

  await capture.start({ source: 'default' });

  assert.equal(requested, false);
  assert.deepEqual(states.at(-1), {
    status: 'error',
    message: 'Camera access is denied in system privacy settings.',
  });
});

test('camera errors map to readable copy', () => {
  assert.match(cameraErrorMessage(domError('NotAllowedError')), /denied/);
  assert.match(cameraErrorMessage(domError('NotReadableError')), /busy/);
  assert.match(cameraErrorMessage(domError('NotFoundError')), /not available/);
  assert.equal(cameraErrorMessage(new Error('Boom')), 'Boom');
  assert.equal(cameraErrorMessage(undefined), 'Camera unavailable.');
});

test('camera constraints pin an exact device only when one is chosen', () => {
  assert.equal('deviceId' in (cameraVideoConstraints() as object), false);
  assert.deepEqual(cameraVideoConstraints('cam-1').deviceId, {
    exact: 'cam-1',
  });
});

test('next and previous device selection wrap and recover from unknown ids', () => {
  const ids = ['a', 'b', 'c'];
  assert.equal(nextCameraDeviceId(ids, 'c', 'next'), 'a');
  assert.equal(nextCameraDeviceId(ids, 'a', 'previous'), 'c');
  assert.equal(nextCameraDeviceId(ids, 'missing', 'next'), 'a');
  assert.equal(nextCameraDeviceId(ids, 'missing', 'previous'), 'c');
  assert.equal(nextCameraDeviceId(['only'], 'only', 'next'), undefined);
});
