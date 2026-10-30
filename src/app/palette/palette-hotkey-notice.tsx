import { Keyboard } from 'lucide-react';
import type { PaletteHotkeyStatus } from './preload-api';
import { shortcutLabel } from './ui';

export function paletteHotkeyNoticeIsVisible(
  status: PaletteHotkeyStatus | null,
  welcomeOffersShortcutSetup: boolean,
): status is PaletteHotkeyStatus {
  if (!status || status.registered) return false;
  return status.configured || !welcomeOffersShortcutSetup;
}

export function paletteHotkeyNoticeCopy(status: PaletteHotkeyStatus) {
  if (!status.configured)
    return {
      state: 'unconfigured',
      title: 'No shortcut set',
      detail: 'Set one to open Nevermind from anywhere.',
      actionLabel: 'Set shortcut',
    } as const;
  return {
    state: 'registration-failed',
    title: `${shortcutLabel(status.accelerator, status.platform)} is unavailable`,
    detail: status.recoveryAccelerator
      ? `Use ${shortcutLabel(status.recoveryAccelerator, status.platform)} to reopen Nevermind.`
      : 'Reopen Nevermind from your app launcher.',
    actionLabel: 'Change shortcut',
  } as const;
}

export function PaletteHotkeyNotice({
  status,
  onSetShortcut,
}: {
  status: PaletteHotkeyStatus;
  onSetShortcut: () => void;
}) {
  const copy = paletteHotkeyNoticeCopy(status);
  return (
    <div
      className="shortcutRegistrationNotice"
      role={copy.state === 'registration-failed' ? 'alert' : 'status'}
      data-state={copy.state}
      title={`${copy.title}. ${copy.detail}`}
    >
      <Keyboard size={14} aria-hidden="true" />
      <p className="shortcutRegistrationMessage">
        <strong>{copy.title}</strong>
        <span>{copy.detail}</span>
      </p>
      <button
        type="button"
        className="shortcutRegistrationButton"
        onClick={onSetShortcut}
      >
        {copy.actionLabel}
      </button>
    </div>
  );
}
