// biome-ignore-all lint: Host-side extension payload boundary keeps dynamic template option shapes by design.
import crypto from 'node:crypto';
import { calculate } from '../search/search-utils';

export type TextTemplateDeps = {
  readClipboardText: () => string | PromiseLike<string>;
  selectedText: () => Promise<unknown>;
};

function templateDateParts(now = new Date()) {
  return {
    date: now.toLocaleDateString(),
    time: now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    datetime: now.toLocaleString(),
  };
}

function normalizeTemplateOptions(input: any = {}) {
  const looksLikeOptions =
    input &&
    typeof input === 'object' &&
    !Array.isArray(input) &&
    [
      'variables',
      'cursorToken',
      'returnCursor',
      'returnResult',
      'includeClipboard',
      'includeSelectedText',
      'promptMissing',
    ].some((key) => key in input);
  return looksLikeOptions
    ? { ...input, variables: input.variables || {} }
    : { variables: input || {} };
}

export function createTextTemplateExpander(deps: TextTemplateDeps) {
  const { readClipboardText, selectedText } = deps;

  async function safeSelectedText() {
    try {
      return String(await selectedText());
    } catch {
      return '';
    }
  }

  async function expandTextTemplate(
    input: string,
    variablesOrOptions: Record<string, unknown> = {},
    hostOptions: { includeClipboard?: boolean } = {},
  ) {
    const options: any = normalizeTemplateOptions(variablesOrOptions);
    const cursorToken = String(
      options.cursorToken || '\uE000NEVERMIND_CURSOR\uE000',
    );
    const missingVariables = new Set<string>();
    const builtins = {
      ...templateDateParts(),
      uuid: crypto.randomUUID(),
      clipboard:
        hostOptions.includeClipboard && options.includeClipboard !== false
          ? await readClipboardText()
          : '',
      selectedText:
        options.includeSelectedText === false ? '' : await safeSelectedText(),
      cursor: cursorToken,
    };
    const values = { ...builtins, ...(options.variables || {}) };
    const textWithCursor = String(input || '').replace(
      /\{\{\s*([^{}]+?)\s*\}\}|\{\s*([^{}]+?)\s*\}/g,
      (_match, doubleName, singleName) => {
        const rawName = String(doubleName || singleName || '').trim();
        if (rawName.startsWith('calculator:')) {
          const result = calculate(rawName.slice('calculator:'.length).trim());
          return result == null ? '' : String(result);
        }
        if (rawName.startsWith('argument:')) {
          const argumentName = rawName.slice('argument:'.length).trim();
          const value = values[argumentName];
          if (value == null) missingVariables.add(argumentName);
          return value == null ? '' : String(value);
        }
        const value = values[rawName];
        if (value == null) missingVariables.add(rawName);
        return value == null ? '' : String(value);
      },
    );
    const cursor = textWithCursor.indexOf(cursorToken);
    const text =
      cursor >= 0 ? textWithCursor.replace(cursorToken, '') : textWithCursor;
    if (options.returnCursor || options.returnResult || options.promptMissing)
      return {
        text,
        cursor: cursor >= 0 ? cursor : undefined,
        missingVariables: Array.from(missingVariables),
      };
    return text;
  }

  return expandTextTemplate;
}
