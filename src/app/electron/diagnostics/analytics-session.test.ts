import assert from 'node:assert/strict';
import test from 'node:test';
import { createAnalyticsSession, newSessionId } from './analytics-session';

const MINUTE = 60_000;

function sessionAt(start: number) {
  let now = start;
  let minted = 0;
  const session = createAnalyticsSession(
    () => now,
    () => `session-${++minted}`,
  );
  return {
    session,
    advance(ms: number) {
      now += ms;
    },
  };
}

test('activity keeps a session alive past the idle window', () => {
  const { session, advance } = sessionAt(1000);
  const first = session.touch();
  for (let step = 0; step < 4; step += 1) {
    advance(20 * MINUTE);
    assert.equal(session.touch(), first);
  }
});

test('thirty idle minutes start a new session', () => {
  const { session, advance } = sessionAt(1000);
  const first = session.touch();
  advance(30 * MINUTE);
  assert.notEqual(session.touch(), first);
});

test('a session ends after twenty-four hours even when active', () => {
  const { session, advance } = sessionAt(1000);
  const first = session.touch();
  for (let step = 0; step < 25; step += 1) {
    advance(59 * MINUTE);
    session.touch();
  }
  assert.notEqual(session.touch(), first);
});

test('ending the session forgets it', () => {
  const { session } = sessionAt(1000);
  const first = session.touch();
  session.end();
  assert.notEqual(session.touch(), first);
});

test('only ending the session changes its epoch', () => {
  const { session, advance } = sessionAt(1_000);
  const epoch = session.epoch();
  session.touch();
  advance(45 * MINUTE);
  session.touch();
  assert.equal(session.epoch(), epoch);
  session.end();
  assert.notEqual(session.epoch(), epoch);
});

test('minted ids are UUIDv7 carrying their start time', () => {
  const now = Date.parse('2026-11-01T12:00:00.000Z');
  const id = newSessionId(now);
  assert.match(
    id,
    /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
  );
  assert.equal(Number.parseInt(id.replace('-', '').slice(0, 12), 16), now);
});
