export const DESIGN_TOKEN_DEFAULTS = {
  '--window-blur-margin': '96px',
  '--search-row-height': '58px',
  '--palette-stack-gap': '8px',
  '--radius-pill': '999px',
  '--radius-xl': '24px',
  '--radius-lg': '16px',
  '--radius-md': '12px',
  '--radius-sm': '8px',
  '--radius-xs': '4px',
  '--fs-2xs': '11px',
  '--fs-xs': '12px',
  '--fs-sm': '13px',
  '--fs-md': '13px',
  '--fs-lg': '14px',
  '--fs-xl': '16px',
  '--fs-2xl': '18px',
  '--surface-1': 'rgba(255, 255, 255, 0.04)',
  '--surface-2': 'rgba(255, 255, 255, 0.06)',
  '--surface-3': 'rgba(255, 255, 255, 0.08)',
  '--surface-4': 'rgba(255, 255, 255, 0.14)',
  '--surface-5': 'rgba(255, 255, 255, 0.18)',
  '--surface-overlay': 'rgba(14, 15, 20, 0.78)',
  '--surface-sunken': 'rgba(14, 15, 20, 0.55)',
  '--surface-extension-window': 'rgb(17, 18, 23)',
  '--surface-extension-window-weak': 'rgba(255, 255, 255, 0.025)',
  '--border-1': 'rgba(255, 255, 255, 0.08)',
  '--border-2': 'rgba(255, 255, 255, 0.12)',
  '--border-3': 'rgba(255, 255, 255, 0.16)',
  '--text-primary': 'rgba(255, 255, 255, 0.92)',
  '--text-secondary': 'rgba(255, 255, 255, 0.72)',
  '--text-tertiary': 'rgba(255, 255, 255, 0.54)',
  '--text-muted': 'rgba(255, 255, 255, 0.42)',
  '--text-strong': '#f5f5fa',
  '--text-1': 'var(--text-primary)',
  '--text-2': 'var(--text-secondary)',
  '--text-3': 'var(--text-tertiary)',
  '--text-dim': 'var(--text-muted)',
  '--link': '#c7b8ff',
  '--item-foreground-yellow': 'rgb(234, 220, 168)',
  '--item-foreground-blue': 'rgb(188, 210, 235)',
  '--item-foreground-purple': 'rgb(211, 197, 235)',
  '--item-foreground-green': 'rgb(194, 222, 195)',
  '--item-foreground-red': 'rgb(232, 191, 191)',
  '--item-foreground-orange': 'rgb(232, 205, 177)',
  '--item-foreground-pink': 'rgb(232, 197, 218)',
  '--accent': '#ffd84d',
  '--accent-strong': 'rgba(255, 216, 77, 0.72)',
  '--accent-mid': 'rgba(255, 216, 77, 0.45)',
  '--accent-soft': 'rgba(255, 216, 77, 0.28)',
  '--accent-faint': 'rgba(255, 216, 77, 0.12)',
  '--danger-text': '#ffb4b4',
  '--danger-bg': 'rgba(255, 90, 90, 0.18)',
  '--font-family-ui':
    '-apple-system, BlinkMacSystemFont, "SF Pro Text", ui-sans-serif, system-ui, "Segoe UI", Roboto, sans-serif',
} as const;

export type DesignTokenName = keyof typeof DESIGN_TOKEN_DEFAULTS;
export type DesignTokenValues = Record<DesignTokenName, string>;
export type DesignTokenOverrides = Partial<DesignTokenValues>;

const LENGTH_TOKENS = new Set<DesignTokenName>(
  Object.keys(DESIGN_TOKEN_DEFAULTS).filter(
    (name) =>
      name.startsWith('--radius-') ||
      name.startsWith('--fs-') ||
      [
        '--window-blur-margin',
        '--search-row-height',
        '--palette-stack-gap',
      ].includes(name),
  ) as DesignTokenName[],
);
const REFERENCE_TOKENS = new Set<DesignTokenName>([
  '--text-1',
  '--text-2',
  '--text-3',
  '--text-dim',
]);
const SAFE_COLOR =
  /^(#[\da-f]{3,8}|rgba?\(\s*\d{1,3}(?:\.\d+)?\s*,\s*\d{1,3}(?:\.\d+)?\s*,\s*\d{1,3}(?:\.\d+)?(?:\s*,\s*(?:0(?:\.\d+)?|1(?:\.0+)?))?\s*\))$/i;
const SAFE_LENGTH = /^(?:0|\d{1,4}(?:\.\d{1,2})?px)$/;
const SAFE_FONT_FAMILY =
  /^(?:[a-z][a-z0-9 -]*|"[a-z][a-z0-9 .-]*")(?:\s*,\s*(?:[a-z][a-z0-9 -]*|"[a-z][a-z0-9 .-]*"))*$/i;

export function validateDesignTokenOverrides(
  input: unknown,
): DesignTokenOverrides {
  if (!input || typeof input !== 'object' || Array.isArray(input)) {
    throw new Error('Design tokens must be an object');
  }
  const entries = Object.entries(input);
  if (entries.length > Object.keys(DESIGN_TOKEN_DEFAULTS).length) {
    throw new Error('Too many design tokens');
  }
  const validated: DesignTokenOverrides = {};
  for (const [name, rawValue] of entries) {
    if (!(name in DESIGN_TOKEN_DEFAULTS))
      throw new Error(`Unknown design token: ${name}`);
    if (typeof rawValue !== 'string' || rawValue.length > 240) {
      throw new Error(`Invalid value for ${name}`);
    }
    const token = name as DesignTokenName;
    const value = rawValue.trim();
    const valid = LENGTH_TOKENS.has(token)
      ? SAFE_LENGTH.test(value)
      : token === '--font-family-ui'
        ? SAFE_FONT_FAMILY.test(value)
        : REFERENCE_TOKENS.has(token)
          ? value === DESIGN_TOKEN_DEFAULTS[token]
          : SAFE_COLOR.test(value);
    if (!valid) throw new Error(`Invalid value for ${name}`);
    if (value !== DESIGN_TOKEN_DEFAULTS[token]) validated[token] = value;
  }
  return validated;
}

export function resolveDesignTokens(input: unknown): DesignTokenValues {
  return { ...DESIGN_TOKEN_DEFAULTS, ...validateDesignTokenOverrides(input) };
}
