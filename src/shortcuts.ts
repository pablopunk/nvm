const DIGIT_CODE = /^Digit\d$/;
const KEY_CODE = /^Key[A-Z]$/;
const WINDOWS_USER_AGENT = /Windows/;
const ARROW_SHORTCUT = /(^|\+)arrow(left|right|up|down)(?=\+|$)/g;

function windowsClearsAltGraphModifiers() {
  return (
    typeof navigator !== 'undefined' &&
    WINDOWS_USER_AGENT.test(navigator.userAgent)
  );
}

export function keyNameForShortcut(event: Pick<KeyboardEvent, 'key' | 'code'>) {
  const key = event.key;
  if (!key || ['Meta', 'Control', 'Alt', 'Shift'].includes(key)) {
    return '';
  }
  if (key === ' ') {
    return 'Space';
  }
  if (key === 'ArrowUp') {
    return 'Up';
  }
  if (key === 'ArrowDown') {
    return 'Down';
  }
  if (key === 'ArrowLeft') {
    return 'Left';
  }
  if (key === 'ArrowRight') {
    return 'Right';
  }
  if (DIGIT_CODE.test(event.code)) {
    return event.code.slice('Digit'.length);
  }
  if (KEY_CODE.test(event.code)) {
    return event.code.slice('Key'.length);
  }
  if (key.length === 1) {
    return key.toUpperCase();
  }
  return key[0].toUpperCase() + key.slice(1);
}

export function acceleratorFromKeyboardEvent(
  event: Pick<
    KeyboardEvent,
    | 'key'
    | 'code'
    | 'metaKey'
    | 'ctrlKey'
    | 'altKey'
    | 'shiftKey'
    | 'getModifierState'
  >,
  recoverAltGraph = windowsClearsAltGraphModifiers(),
) {
  const key = keyNameForShortcut(event);
  if (!key) {
    return '';
  }
  const altGraph =
    recoverAltGraph && (event.getModifierState?.('AltGraph') ?? false);
  const parts: string[] = [];
  if (event.metaKey) {
    parts.push('Command');
  }
  if (event.ctrlKey || altGraph) {
    parts.push('Control');
  }
  if (event.altKey || altGraph) {
    parts.push('Alt');
  }
  if (event.shiftKey) {
    parts.push('Shift');
  }
  parts.push(key);
  const isArrow =
    key === 'Left' || key === 'Right' || key === 'Up' || key === 'Down';
  if (parts.length < 2 && !key.startsWith('F') && !isArrow) {
    return '';
  }
  return parts.join('+');
}

export function normalizedShortcut(value?: string) {
  return String(value || '')
    .replace(/\s+/g, '')
    .toLowerCase()
    .replace(ARROW_SHORTCUT, '$1$2');
}
