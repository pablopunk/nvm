import crypto from 'node:crypto';

interface SelectedTextReaderDependencies<Snapshot, Target> {
  selectionTarget(): Target | null | Promise<Target | null>;
  readAccessibilityText(target: Target): Promise<string | null | undefined>;
  paletteIsFocused(): boolean;
  clipboardSnapshot(): Snapshot;
  readClipboardText(): string;
  writeClipboardText(text: string): void;
  restoreClipboardSnapshot(snapshot: Snapshot): void;
  copySelectionIntoClipboard(target: Target): Promise<boolean>;
  concealClipboardText(text: string): void;
  selectionRead?(result: {
    length: number;
    method: 'accessibility' | 'clipboard' | 'none';
  }): void;
  delay?(durationMs: number): Promise<void>;
  sentinel?(): string;
}

const CLIPBOARD_POLL_INTERVAL_MS = 25;
const CLIPBOARD_POLL_ATTEMPTS = 20;
const ACCESSIBILITY_POLL_ATTEMPTS = 8;

async function waitForClipboardText(
  readText: () => string,
  concealText: (text: string) => void,
  sentinel: string,
  delay: (durationMs: number) => Promise<void>,
  attemptsLeft: number,
): Promise<string | null> {
  const text = readText();
  if (text !== sentinel) {
    concealText(text);
    return text || null;
  }
  if (attemptsLeft <= 1) {
    return null;
  }
  await delay(CLIPBOARD_POLL_INTERVAL_MS);
  return waitForClipboardText(
    readText,
    concealText,
    sentinel,
    delay,
    attemptsLeft - 1,
  );
}

export function createSelectedTextReader<Snapshot, Target>(
  dependencies: SelectedTextReaderDependencies<Snapshot, Target>,
) {
  let pending: Promise<string | null> | null = null;
  const delay =
    dependencies.delay ??
    ((durationMs: number) =>
      new Promise<void>((resolve) => setTimeout(resolve, durationMs)));

  async function readAccessibilitySelection(
    target: Target,
    attemptsLeft: number,
  ) {
    if (dependencies.paletteIsFocused()) {
      return null;
    }
    const text = String(
      (await dependencies.readAccessibilityText(target)) ?? '',
    );
    if (text || attemptsLeft <= 1) {
      return text || null;
    }
    await delay(CLIPBOARD_POLL_INTERVAL_MS);
    return readAccessibilitySelection(target, attemptsLeft - 1);
  }

  async function readClipboardSelection(target: Target) {
    const snapshot = dependencies.clipboardSnapshot();
    const sentinel =
      dependencies.sentinel?.() ??
      `__NEVERMIND_SELECTION_${crypto.randomUUID()}__`;
    dependencies.concealClipboardText(sentinel);
    dependencies.writeClipboardText(sentinel);
    try {
      if (!(await dependencies.copySelectionIntoClipboard(target))) {
        return null;
      }
      return waitForClipboardText(
        dependencies.readClipboardText,
        dependencies.concealClipboardText,
        sentinel,
        delay,
        CLIPBOARD_POLL_ATTEMPTS,
      );
    } finally {
      dependencies.restoreClipboardSnapshot(snapshot);
    }
  }

  async function read() {
    const target = await dependencies.selectionTarget();
    if (!target) {
      return null;
    }
    const accessibilityText = await readAccessibilitySelection(
      target,
      ACCESSIBILITY_POLL_ATTEMPTS,
    );
    if (accessibilityText) {
      dependencies.selectionRead?.({
        length: accessibilityText.length,
        method: 'accessibility',
      });
      return accessibilityText;
    }
    if (dependencies.paletteIsFocused()) {
      return null;
    }
    const clipboardText = await readClipboardSelection(target);
    dependencies.selectionRead?.({
      length: clipboardText?.length || 0,
      method: clipboardText ? 'clipboard' : 'none',
    });
    return clipboardText;
  }

  return function selectedText() {
    if (!pending) {
      pending = read().finally(() => (pending = null));
    }
    return pending;
  };
}
