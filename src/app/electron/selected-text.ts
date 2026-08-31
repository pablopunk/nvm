import crypto from 'node:crypto';

type SelectedTextReaderDependencies<Snapshot> = {
  readAccessibilityText(): Promise<string | null | undefined>;
  paletteIsFocused(): boolean;
  clipboardSnapshot(): Snapshot;
  readClipboardText(): string;
  writeClipboardText(text: string): void;
  restoreClipboardSnapshot(snapshot: Snapshot): void;
  copySelectionIntoClipboard(): Promise<boolean>;
  concealClipboardText(text: string): void;
  delay?(durationMs: number): Promise<void>;
  sentinel?(): string;
};

const CLIPBOARD_POLL_INTERVAL_MS = 25;
const CLIPBOARD_POLL_ATTEMPTS = 20;
const ACCESSIBILITY_POLL_ATTEMPTS = 8;

export function createSelectedTextReader<Snapshot>(
  dependencies: SelectedTextReaderDependencies<Snapshot>,
) {
  let pending: Promise<string | null> | null = null;
  const delay =
    dependencies.delay ??
    ((durationMs: number) =>
      new Promise<void>((resolve) => setTimeout(resolve, durationMs)));

  async function readAccessibilitySelection(attemptsLeft: number) {
    if (dependencies.paletteIsFocused()) return null;
    const text = String((await dependencies.readAccessibilityText()) ?? '');
    if (text || attemptsLeft <= 1) return text || null;
    await delay(CLIPBOARD_POLL_INTERVAL_MS);
    return readAccessibilitySelection(attemptsLeft - 1);
  }

  async function read() {
    const accessibilityText = await readAccessibilitySelection(
      ACCESSIBILITY_POLL_ATTEMPTS,
    );
    if (accessibilityText) return accessibilityText;
    if (dependencies.paletteIsFocused()) return null;

    const snapshot = dependencies.clipboardSnapshot();
    const sentinel =
      dependencies.sentinel?.() ??
      `__NEVERMIND_SELECTION_${crypto.randomUUID()}__`;
    dependencies.concealClipboardText(sentinel);
    dependencies.writeClipboardText(sentinel);
    try {
      if (!(await dependencies.copySelectionIntoClipboard())) return null;
      for (let attempt = 0; attempt < CLIPBOARD_POLL_ATTEMPTS; attempt += 1) {
        const text = dependencies.readClipboardText();
        if (text !== sentinel) {
          dependencies.concealClipboardText(text);
          return text || null;
        }
        await delay(CLIPBOARD_POLL_INTERVAL_MS);
      }
      return null;
    } finally {
      dependencies.restoreClipboardSnapshot(snapshot);
    }
  }

  return function selectedText() {
    if (!pending) pending = read().finally(() => (pending = null));
    return pending;
  };
}
