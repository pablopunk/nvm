import crypto from 'node:crypto';
import type { MaybePromise } from './electron-clipboard';

interface SelectedTextReaderDependencies<Snapshot, Target> {
  selectionTarget(): Target | null | Promise<Target | null>;
  readAccessibilityText(target: Target): Promise<string | null | undefined>;
  paletteIsFocused(): boolean;
  clipboardSnapshot(): MaybePromise<Snapshot>;
  validateClipboardSnapshot?(snapshot: Snapshot): void;
  readClipboardText(): MaybePromise<string>;
  writeClipboardText(text: string): MaybePromise<void>;
  restoreClipboardSnapshot(snapshot: Snapshot): MaybePromise<void>;
  copySelectionIntoClipboard(target: Target): Promise<boolean>;
  concealClipboardText(text: string): void;
  selectionRead?(result: {
    method: 'accessibility' | 'clipboard';
    length: number;
  }): void;
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
    dependencies.validateClipboardSnapshot?.(snapshot);
    const sentinel =
      dependencies.sentinel?.() ??
      `__NEVERMIND_SELECTION_${crypto.randomUUID()}__`;
    dependencies.concealClipboardText(sentinel);
    try {
      await dependencies.writeClipboardText(sentinel);
      if (!(await dependencies.copySelectionIntoClipboard(target))) {
        throw new Error(
          'Could not copy selected text. Select it and try again.',
        );
      }
      return await waitForClipboardText({
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
      throw new Error(
        'Could not identify the source app. Select the text and try again.',
      );
    }
    if (dependencies.paletteIsFocused()) {
      throw new Error(
        'Nevermind still has focus. Return to the source app and try again.',
      );
    }
    const accessibilityText = String(
      (await dependencies.readAccessibilityText(target)) ?? '',
    );
    if (accessibilityText) {
      dependencies.selectionRead?.({
        method: 'accessibility',
        length: accessibilityText.length,
      });
      return accessibilityText;
    }
    if (dependencies.paletteIsFocused()) {
      throw new Error(
        'Nevermind regained focus. Return to the source app and try again.',
      );
    }
    const text = await readClipboardSelection(target);
    dependencies.selectionRead?.({
      method: 'clipboard',
      length: text?.length ?? 0,
    });
    return text;
  }

  return function selectedText() {
    if (!pending) {
      pending = read().finally(() => (pending = null));
    }
    return pending;
  };
}
