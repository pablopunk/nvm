/** Presentation capabilities do not grant action execution authority. */
export class PaneInvocations {
  private records = new Map<string, { senderId: number; createdAt: number }>();

  register(requestId: string, senderId: number, now = Date.now()) {
    this.prune(now);
    this.records.set(requestId, { senderId, createdAt: now });
  }

  owns(context: unknown, senderId?: number, now = Date.now()) {
    this.prune(now);
    if (!context || typeof context !== 'object') return false;
    const requestId = (context as { requestId?: unknown }).requestId;
    if (typeof requestId !== 'string') return false;
    return (
      this.records.get(requestId)?.senderId === senderId &&
      senderId !== undefined
    );
  }

  invalidate(senderId: number) {
    for (const [id, record] of this.records)
      if (record.senderId === senderId) this.records.delete(id);
  }

  private prune(now: number) {
    for (const [id, record] of this.records)
      if (now - record.createdAt > 24 * 60 * 60 * 1000) this.records.delete(id);
    while (this.records.size > 1000) {
      const oldest = this.records.keys().next().value;
      if (oldest) this.records.delete(oldest);
    }
  }
}
