import type {
  ExtensionContext,
  ExtensionFormField,
  ExtensionFormValue,
  ExtensionView,
} from '../resources/nevermind-extension-api';
import {
  applyWindowSpecWithSettings,
  WINDOW_MANAGEMENT_SUBTITLE,
} from './window-management-commands';
import {
  EXACT_UNIT_OPTIONS,
  type ExactDimensionErrors,
  type ExactDimensionKey,
  type ExactDimensionUnit,
  exactDimensionParseFailed,
  exactWindowLayoutSpec,
  parseExactDimensions,
} from './window-management-exact-values';
import { windowManagementSettingsAction } from './window-management-settings-action';

interface ExactWindowPresentation {
  id: string;
  title: string;
  submitTitle: string;
  icon: string;
  keywords: string[];
  defaultUnit: ExactDimensionUnit;
  keys: [ExactDimensionKey, ExactDimensionKey];
  labels: Partial<Record<ExactDimensionKey, string>>;
  descriptions: Partial<Record<ExactDimensionKey, string>>;
}

const UNIT_FIELD_ID = 'unit';

const KEEP_CURRENT_PLACEHOLDER = 'Blank keeps current';

function unitFieldFor(
  presentation: ExactWindowPresentation,
  values: Record<string, ExtensionFormValue>,
): ExtensionFormField {
  return {
    id: UNIT_FIELD_ID,
    label: 'Unit',
    type: 'dropdown',
    value: String(values[UNIT_FIELD_ID] ?? presentation.defaultUnit),
    options: EXACT_UNIT_OPTIONS,
    description: 'Applies to both values below.',
  };
}

function dimensionFieldFor(
  presentation: ExactWindowPresentation,
  key: ExactDimensionKey,
  values: Record<string, ExtensionFormValue>,
  errors: ExactDimensionErrors,
): ExtensionFormField {
  return {
    id: key,
    label: presentation.labels[key],
    type: 'number',
    value: String(values[key] ?? ''),
    placeholder: KEEP_CURRENT_PLACEHOLDER,
    description: presentation.descriptions[key],
    error: errors[key],
  };
}

function exactWindowInputs(
  kind: ExactWindowKind,
  values: Record<string, ExtensionFormValue>,
) {
  const unit = values[UNIT_FIELD_ID];
  return Object.fromEntries(
    EXACT_WINDOW_PRESENTATION[kind].keys.map((key) => [
      key,
      { text: values[key], unit },
    ]),
  );
}

const EXACT_WINDOW_HINT =
  'Blank keeps the current value. Values are clamped inside the work area.';

async function submitExactWindow(
  ctx: ExtensionContext,
  kind: ExactWindowKind,
  formValues: Record<string, ExtensionFormValue> = {},
) {
  const presentation = EXACT_WINDOW_PRESENTATION[kind];
  const parsed = parseExactDimensions(exactWindowInputs(kind, formValues));
  if (exactDimensionParseFailed(parsed)) {
    return {
      view: exactWindowPromptView(ctx, kind, {
        values: formValues,
        errors: parsed.errors,
        notice: parsed.message,
      }),
      navigation: 'replace' as const,
    };
  }
  await applyWindowSpecWithSettings(ctx, presentation.title, () =>
    exactWindowLayoutSpec(parsed.dimensions),
  );
  return;
}

function exactWindowSubmitAction(ctx: ExtensionContext, kind: ExactWindowKind) {
  return ctx.actions.run(
    EXACT_WINDOW_PRESENTATION[kind].submitTitle,
    (innerCtx, action) => submitExactWindow(innerCtx, kind, action.formValues),
    { dismissAfterRun: 'auto' },
  );
}

export type ExactWindowKind = 'move' | 'resize';

export const EXACT_WINDOW_PRESENTATION: Record<
  ExactWindowKind,
  ExactWindowPresentation
> = {
  move: {
    id: 'move-window',
    title: 'Move Window…',
    submitTitle: 'Move Window',
    icon: 'move',
    keywords: ['position', 'exact', 'coordinates', 'offset', 'place'],
    defaultUnit: 'points',
    keys: ['x', 'y'],
    labels: { x: 'X offset', y: 'Y offset' },
    descriptions: {
      x: 'Distance from the left edge of the work area.',
      y: 'Distance from the top edge of the work area.',
    },
  },
  resize: {
    id: 'resize-window',
    title: 'Resize Window…',
    submitTitle: 'Resize Window',
    icon: 'scaling',
    keywords: ['size', 'exact', 'dimensions', 'width', 'height'],
    defaultUnit: 'percent',
    keys: ['width', 'height'],
    labels: { width: 'Width', height: 'Height' },
    descriptions: {
      width: 'Width of the window. Percent is of the work area width.',
      height: 'Height of the window. Percent is of the work area height.',
    },
  },
};

export function exactWindowFields(
  kind: ExactWindowKind,
  values: Record<string, ExtensionFormValue> = {},
  errors: ExactDimensionErrors = {},
): ExtensionFormField[] {
  const presentation = EXACT_WINDOW_PRESENTATION[kind];
  return [
    unitFieldFor(presentation, values),
    ...presentation.keys.map((key) =>
      dimensionFieldFor(presentation, key, values, errors),
    ),
  ];
}

export interface ExactWindowPromptState {
  values?: Record<string, ExtensionFormValue>;
  errors?: ExactDimensionErrors;
  notice?: string;
}

export function exactWindowPromptView(
  ctx: ExtensionContext,
  kind: ExactWindowKind,
  state: ExactWindowPromptState = {},
): ExtensionView {
  const presentation = EXACT_WINDOW_PRESENTATION[kind];
  const { values = {}, errors = {}, notice } = state;
  return {
    type: 'prompt',
    title: presentation.title,
    subtitle: notice ?? EXACT_WINDOW_HINT,
    fields: exactWindowFields(kind, values, errors),
    submitAction: exactWindowSubmitAction(ctx, kind),
  };
}

export function exactWindowAction(
  ctx: ExtensionContext,
  kind: ExactWindowKind,
) {
  const presentation = EXACT_WINDOW_PRESENTATION[kind];
  return {
    id: presentation.id,
    title: presentation.title,
    subtitle: WINDOW_MANAGEMENT_SUBTITLE,
    icon: presentation.icon,
    keywords: presentation.keywords,
    actions: [windowManagementSettingsAction(ctx)],
    action: ctx.input.prompt({
      title: presentation.title,
      message: EXACT_WINDOW_HINT,
      fields: exactWindowFields(kind),
      submitTitle: presentation.submitTitle,
      action: exactWindowSubmitAction(ctx, kind),
    }),
  };
}

export function exactWindowActions(ctx: ExtensionContext) {
  return [exactWindowAction(ctx, 'move'), exactWindowAction(ctx, 'resize')];
}
