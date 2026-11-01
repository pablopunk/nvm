/** Design tokens shared with sandboxed webviews as `--nvm-*` variables; elements stay unstyled. */
export const WEBVIEW_THEME_TOKENS = [
  'surface-canvas',
  'surface-panel',
  'surface-raised',
  'surface-interactive',
  'surface-selected',
  'surface-sunken',
  'text-strong',
  'text-primary',
  'text-secondary',
  'text-muted',
  'accent',
  'accent-fill',
  'accent-contrast',
  'accent-soft',
  'radius-xs',
  'radius-sm',
  'radius-md',
  'radius-lg',
  'radius-pill',
] as const;

export type WebviewThemeValues = Record<string, string>;

export function readWebviewThemeValues(
  computed: Pick<CSSStyleDeclaration, 'getPropertyValue' | 'fontFamily'>,
): WebviewThemeValues {
  const values: WebviewThemeValues = {
    'font-sans': computed.fontFamily,
  };
  for (const token of WEBVIEW_THEME_TOKENS)
    values[token] = computed.getPropertyValue(`--${token}`).trim();
  return values;
}

export function webviewThemeStyle(values: WebviewThemeValues) {
  const declarations = Object.entries(values)
    .filter(([, value]) => value)
    .map(([token, value]) => `--nvm-${token}:${value.replace(/[<;{}]/g, '')};`)
    .join('');
  return `<style data-nevermind-theme>:root{color-scheme:light dark;scrollbar-width:none;${declarations}}::-webkit-scrollbar{display:none}</style>`;
}

/** Inserts after `<head>` when present so a leading doctype keeps standards mode. */
export function withWebviewTheme(html: string, values: WebviewThemeValues) {
  if (!Object.keys(values).length) return html;
  const style = webviewThemeStyle(values);
  const head = html.match(/<head[^>]*>/i);
  if (!head || head.index === undefined) return `${style}${html}`;
  const insertAt = head.index + head[0].length;
  return `${html.slice(0, insertAt)}${style}${html.slice(insertAt)}`;
}

/** Server rendering and tests have no document; webviews then get no theme. */
export function currentWebviewThemeValues(): WebviewThemeValues {
  if (typeof document === 'undefined' || typeof getComputedStyle !== 'function')
    return {};
  return readWebviewThemeValues(getComputedStyle(document.documentElement));
}
