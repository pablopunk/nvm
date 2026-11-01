import { randomBytes } from 'node:crypto';

const SESSION_IDLE_MS = 30 * 60 * 1000;
const SESSION_MAX_MS = 24 * 60 * 60 * 1000;
const UUID_TIMESTAMP_BYTES = 6;

/** A UUIDv7 carries its start time, which PostHog uses to place the session. */
export function newSessionId(now: number) {
  const bytes = randomBytes(16);
  bytes.writeUIntBE(now, 0, UUID_TIMESTAMP_BYTES);
  bytes[6] = (bytes[6] & 0x0f) | 0x70;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = bytes.toString('hex');
  return [
    hex.slice(0, 8),
    hex.slice(8, 12),
    hex.slice(12, 16),
    hex.slice(16, 20),
    hex.slice(20),
  ].join('-');
}

/**
 * Owns the analytics session for the whole app. A session continues while the
 * person keeps doing meaningful things (viewing, running commands) and starts
 * over after 30 idle minutes or 24 hours, matching how PostHog reads sessions.
 */
export function createAnalyticsSession(
  clock: () => number = Date.now,
  mintId: (now: number) => string = newSessionId,
) {
  let current: { id: string; startedAt: number; lastActiveAt: number } | null =
    null;
  return {
    /** Records activity and returns the session it belongs to. */
    touch() {
      const now = clock();
      if (
        !current ||
        now - current.lastActiveAt >= SESSION_IDLE_MS ||
        now - current.startedAt >= SESSION_MAX_MS
      ) {
        current = { id: mintId(now), startedAt: now, lastActiveAt: now };
      }
      current.lastActiveAt = now;
      return current.id;
    },
    /** Forgets the session, so withdrawn consent leaves nothing to resume. */
    end() {
      current = null;
    },
  };
}
