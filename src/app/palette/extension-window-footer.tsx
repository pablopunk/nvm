import { actionsShortcutHint } from './actions-shortcut-hint';

export interface ExtensionWindowFooterProps {
  progress?: string;
  showOpen: boolean;
  showActions: boolean;
  showBack: boolean;
}

export function ExtensionWindowFooter({
  progress,
  showOpen,
  showActions,
  showBack,
}: ExtensionWindowFooterProps) {
  return (
    <footer className="extensionWindowFooter">
      {progress ? <span>{progress}</span> : null}
      {showOpen ? (
        <span>
          <kbd>↵</kbd> Open
        </span>
      ) : null}
      {showActions ? (
        <span>
          <kbd>{actionsShortcutHint()}</kbd> Actions
        </span>
      ) : null}
      {showBack ? (
        <span>
          <kbd>esc</kbd> Back
        </span>
      ) : null}
    </footer>
  );
}
