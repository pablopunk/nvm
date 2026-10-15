export type FreeSpendCapNotice = {
  kind: 'free_spend_cap_reached';
  title: string;
  message: string;
  resetsAt?: string;
};

export function freeSpendCapNoticeFromText(
  text: string,
): FreeSpendCapNotice | null {
  if (!/free[_ -]?spend[_ -]?cap[_ -]?reached/i.test(text)) return null;
  const resetsAt = text.match(/"resetsAt"\s*:\s*"([^"]+)"/i)?.[1];
  return {
    kind: 'free_spend_cap_reached',
    title: 'Free AI temporarily paused',
    message: resetsAt
      ? `Free-pool requests are temporarily paused. Capacity may return earlier as in-flight reservations settle or release. The current period resets at ${resetsAt}.`
      : 'Free-pool requests are temporarily paused. Capacity may return earlier as in-flight reservations settle or release.',
    ...(resetsAt ? { resetsAt } : {}),
  };
}
