import crypto from 'node:crypto';
import type { MaybePromise } from './electron-clipboard';

interface SelectedTextReaderDependencies<Snapshot, Target> {
  selectionTarget(): Target | null | Promise<Target | null>;
  readAccessibilityText(target: Target): Promise<string | null | undefined>;
  paletteIsFocused(): boolean;
  clipboardSnapshot(): MaybePromise<Snapshot>;
  readClipboardText(): MaybePromise<string>;
  writeClipboardText(text: string): MaybePromise<void>;
  restoreClipboardSnapshot(snapshot: Snapshot): MaybePromise<void>;
  copySelectionIntoClipboard(target: Target): Promise<boolean>;
  concealClipboardText(text: string): void;
  delay?(durationMs: number): Promise<void>;
  sentinel?(): string;
}

const CLIPBOARD_POLL_INTERVAL_MS = 25;
const CLIPBOARD_POLL_ATTEMPTS = 20;

interface ClipboardPoll {
  attemptsLeft: number;
  concealText(text: string): void;
  delay(durationMs: number): Promise<void>;
  readText(): MaybePromise<string>;
  sentinel: string;
}

async function waitForClipboardText(
  poll: ClipboardPoll,
): Promise<string | null> {
  const text = await poll.readText();
  if (text !== poll.sentinel) {
    poll.concealText(text);
    return text || null;
  }
  if (poll.attemptsLeft <= 1) {
    return null;
  }
  await poll.delay(CLIPBOARD_POLL_INTERVAL_MS);
  return waitForClipboardText({
    ...poll,
    attemptsLeft: poll.attemptsLeft - 1,
  });
}

export function createSelectedTextReader<Snapshot, Target>(
  dependencies: SelectedTextReaderDependencies<Snapshot, Target>,
) {
  let pending: Promise<string | null> | null = null;
  const delay =
    dependencies.delay ??
    ((durationMs: number) =>
      new Promise<void>((resolve) => setTimeout(resolve, durationMs)));

  async function readClipboardSelection(target: Target) {
    const snapshot = await dependencies.clipboardSnapshot();
    const sentinel =
      dependencies.sentinel?.() ??
      `__NEVERMIND_SELECTION_${crypto.randomUUID()}__`;
    dependencies.concealClipboardText(sentinel);
    await dependencies.writeClipboardText(sentinel);
    try {
      if (!(await dependencies.copySelectionIntoClipboard(target))) {
        return null;
      }
      return waitForClipboardText({
        attemptsLeft: CLIPBOARD_POLL_ATTEMPTS,
        concealText: dependencies.concealClipboardText,
        delay,
        readText: dependencies.readClipboardText,
        sentinel,
      });
    } finally {
      await dependencies.restoreClipboardSnapshot(snapshot);
    }
  }

  async function read() {
    const target = await dependencies.selectionTarget();
    if (!target) {
      return null;
    }
    if (dependencies.paletteIsFocused()) {
      return null;
    }
    const accessibilityText = String(
      (await dependencies.readAccessibilityText(target)) ?? '',
    );
    if (accessibilityText) {
      return accessibilityText;
    }
    if (dependencies.paletteIsFocused()) {
      return null;
    }
    return readClipboardSelection(target);
  }

  return function selectedText() {
    if (!pending) {
      pending = read().finally(() => (pending = null));
    }
    return pending;
  };
}
