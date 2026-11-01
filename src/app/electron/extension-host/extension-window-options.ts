// biome-ignore-all lint: Window options arrive from extensions and are validated structurally.

export const EXTENSION_WINDOW_OPTION_DEFAULTS = Object.freeze({
  titleBar: 'default',
  chrome: 'default',
  size: 'default',
  width: 560,
  height: 420,
  alwaysOnTop: true,
  visibleOnAllSpaces: false,
  hideOnBlur: false,
  persistent: false,
  remembersFrame: false,
  focusable: true,
  showInactive: false,
  ignoreMouseEvents: false,
  position: 'center',
} as const);

const WINDOW_OPTION_KEYS = new Set([
  'id',
  'restoreKey',
  'title',
  'titleBar',
  'chrome',
  'width',
  'height',
  'size',
  'alwaysOnTop',
  'visibleOnAllSpaces',
  'hideOnBlur',
  'focusable',
  'showInactive',
  'ignoreMouseEvents',
  'position',
  'persistent',
  'remembersFrame',
]);
const STABLE_KEY = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;
export const PASSIVE_WINDOW_MIN_WIDTH = 160;

function invalidWindowInput(message: string): never {
  throw new Error(`Invalid extension window input: ${message}`);
}

function finiteDimension(value: unknown, fallback: number, name: string) {
  if (value === undefined) return fallback;
  if (typeof value !== 'number' || !Number.isFinite(value))
    invalidWindowInput(`${name} must be a finite number`);
  return Math.round(value);
}

/** Validate only the bounded window-options structure, never ExtensionView itself. */
export function normalizeExtensionWindowOptions(value: unknown) {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    invalidWindowInput('options must be an object');
  const options = value as Record<string, unknown>;
  for (const key of Object.keys(options)) {
    if (!WINDOW_OPTION_KEYS.has(key))
      invalidWindowInput(`unknown option ${key}`);
  }
  for (const key of ['id', 'restoreKey'] as const) {
    if (options[key] !== undefined) {
      if (typeof options[key] !== 'string' || !STABLE_KEY.test(options[key]))
        invalidWindowInput(`${key} must be a bounded stable key`);
    }
  }
  if (options.title !== undefined && typeof options.title !== 'string')
    invalidWindowInput('title must be a string');
  if (
    options.titleBar !== undefined &&
    options.titleBar !== 'default' &&
    options.titleBar !== 'hidden'
  )
    invalidWindowInput('titleBar must be default or hidden');
  if (
    options.chrome !== undefined &&
    options.chrome !== 'default' &&
    options.chrome !== 'none'
  )
    invalidWindowInput('chrome must be default or none');
  if (
    options.size !== undefined &&
    options.size !== 'default' &&
    options.size !== 'large'
  )
    invalidWindowInput('size must be default or large');
  for (const key of [
    'alwaysOnTop',
    'visibleOnAllSpaces',
    'hideOnBlur',
    'focusable',
    'showInactive',
    'ignoreMouseEvents',
    'persistent',
    'remembersFrame',
  ]) {
    if (options[key] !== undefined && typeof options[key] !== 'boolean')
      invalidWindowInput(`${key} must be a boolean`);
  }
  if (
    options.position !== undefined &&
    !['center', 'top-center', 'bottom-center', 'menu-bar'].includes(
      String(options.position),
    )
  )
    invalidWindowInput(
      'position must be center, top-center, bottom-center, or menu-bar',
    );
  finiteDimension(
    options.width,
    EXTENSION_WINDOW_OPTION_DEFAULTS.width,
    'width',
  );
  finiteDimension(
    options.height,
    EXTENSION_WINDOW_OPTION_DEFAULTS.height,
    'height',
  );
  return { ...options };
}

export function extensionWindowSize(options: any = {}) {
  const large = options.size === 'large';
  const passive = options.focusable === false;
  return {
    width: Math.max(
      passive ? PASSIVE_WINDOW_MIN_WIDTH : 320,
      Math.min(
        1600,
        finiteDimension(options.width, large ? 900 : 560, 'width'),
      ),
    ),
    height: Math.max(
      passive ? 64 : 240,
      Math.min(
        1200,
        finiteDimension(options.height, large ? 680 : 420, 'height'),
      ),
    ),
  };
}

export function extensionWindowId(
  view: any,
  options: any = {},
  hash = defaultHashValue,
) {
  return String(
    options.id ||
      view?.id ||
      `window:${hash(`${view?.title || 'Extension Window'}:${JSON.stringify(view || {})}`)}`,
  );
}

function defaultHashValue(value: string) {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1)
    hash = (Math.imul(31, hash) + value.charCodeAt(index)) | 0;
  return String(Math.abs(hash));
}
