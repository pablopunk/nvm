import type { AiChatEvent } from './preload-api';

/** Unscoped legacy events are routable only while their initiator is unambiguous. */
export class AiChatEventOwners {
  private traces = new Map<string, string>();
  private pending = new Set<string>();
  private latest = new Map<string, string | undefined>();

  begin(sessionId: string, traceId?: string) {
    this.pending.add(sessionId);
    this.latest.set(sessionId, traceId);
    if (traceId) this.traces.set(traceId, sessionId);
  }
  sessionFor(event: AiChatEvent) {
    if (event.chatId) return event.chatId;
    if (event.traceId) return this.traces.get(event.traceId);
    return this.pending.size === 1
      ? this.pending.values().next().value
      : undefined;
  }
  finish(event: AiChatEvent) {
    const session = this.sessionFor(event);
    if (session && ['done', 'error', 'aborted'].includes(event.type))
      this.fail(session, event.traceId);
  }
  fail(sessionId: string, traceId?: string) {
    if (traceId && this.latest.get(sessionId) !== traceId) {
      this.traces.delete(traceId);
      return;
    }
    this.release(sessionId);
  }
  release(sessionId: string) {
    this.pending.delete(sessionId);
    this.latest.delete(sessionId);
    for (const [traceId, owner] of this.traces)
      if (owner === sessionId) this.traces.delete(traceId);
  }
}
