import type {
  ExtensionContext,
  ExtensionFormField,
  ExtensionFormValue,
  ExtensionView,
} from '../resources/nevermind-extension-api';
import {
  type CustomWindowCommand,
  type CustomWindowCommandsStore,
  createCustomWindowCommandId,
  MAX_CUSTOM_WINDOW_COMMAND_TITLE_LENGTH,
  MAX_CUSTOM_WINDOW_COMMANDS,
} from './window-management-custom-store';
import {
  EXACT_DIMENSION_KEYS,
  EXACT_UNIT_OPTIONS,
  type ExactDimensionErrors,
  type ExactDimensionInput,
  type ExactDimensionKey,
  type ExactDimensionValues,
  exactDimensionParseFailed,
  parseExactDimensions,
} from './window-management-exact-values';

const TITLE_FIELD_ID = 'title';

const FORM_HINT =
  'Offsets are from the work-area top-left. Blank keeps the current value.';

function dimensionFieldsFor(
  key: ExactDimensionKey,
  values: Record<string, ExtensionFormValue>,
  errors: CustomCommandFormErrors,
): ExtensionFormField[] {
  return [
    {
      id: key,
      label: DIMENSION_LABELS[key],
      type: 'number',
      value: String(values[key] ?? ''),
      placeholder: 'Blank keeps current',
      error: errors[key],
    },
    {
      id: unitFieldIdFor(key),
      label: `${DIMENSION_LABELS[key]} unit`,
      type: 'dropdown',
      value: String(values[unitFieldIdFor(key)] ?? 'points'),
      options: EXACT_UNIT_OPTIONS,
    },
  ];
}

function customDimensionInputs(formValues: Record<string, ExtensionFormValue>) {
  const inputs: Partial<Record<ExactDimensionKey, ExactDimensionInput>> = {};
  for (const key of EXACT_DIMENSION_KEYS) {
    inputs[key] = {
      text: formValues[key],
      unit: formValues[unitFieldIdFor(key)],
    };
  }
  return inputs;
}

function customCommandTitleError(title: string) {
  if (!title) {
    return 'Enter a title.';
  }
  if (title.length > MAX_CUSTOM_WINDOW_COMMAND_TITLE_LENGTH) {
    return `Use ${MAX_CUSTOM_WINDOW_COMMAND_TITLE_LENGTH} characters or fewer.`;
  }
  return;
}

function customCommandLimitError(commandCount: number) {
  return commandCount >= MAX_CUSTOM_WINDOW_COMMANDS
    ? `Limit reached: ${MAX_CUSTOM_WINDOW_COMMANDS} custom commands.`
    : undefined;
}

function nextCustomWindowCommands(
  commands: CustomWindowCommand[],
  command: CustomWindowCommand,
  editingId: string | undefined,
) {
  if (!editingId) {
    return [...commands, command];
  }
  return commands.map((existing) =>
    existing.id === editingId ? command : existing,
  );
}

function validatedCustomCommand(
  formValues: Record<string, ExtensionFormValue>,
  editingId: string | undefined,
  commandCount: number,
) {
  const title = String(formValues[TITLE_FIELD_ID] ?? '').trim();
  const parsed = parseExactDimensions(customDimensionInputs(formValues));
  const errors: CustomCommandFormErrors = { ...parsed.errors };
  const titleError = customCommandTitleError(title);
  const limitError = editingId
    ? undefined
    : customCommandLimitError(commandCount);
  if (titleError || limitError) {
    errors.title = titleError ?? limitError;
  }
  const failed =
    exactDimensionParseFailed(parsed) || errors.title !== undefined;
  return {
    failed,
    errors,
    notice: parsed.message,
    title,
    dimensions: parsed.dimensions,
  };
}

async function saveCustomWindowCommand(
  ctx: ExtensionContext,
  store: CustomWindowCommandsStore,
  editingId: string | undefined,
  formValues: Record<string, ExtensionFormValue>,
) {
  const commands = await store.load();
  const validation = validatedCustomCommand(
    formValues,
    editingId,
    commands.length,
  );
  if (validation.failed) {
    return {
      view: customWindowCommandFormView(ctx, store, {
        editingId,
        values: formValues,
        errors: validation.errors,
        notice: validation.notice,
      }),
      navigation: 'replace' as const,
    };
  }
  const command = customCommandFromValidated(
    validation.title,
    validation.dimensions,
    editingId,
  );
  await store.save(nextCustomWindowCommands(commands, command, editingId));
  ctx.views.invalidate();
  return { navigation: 'pop' as const, action: ctx.views.refresh() };
}

function customCommandFromValidated(
  title: string,
  dimensions: ExactDimensionValues,
  editingId: string | undefined,
): CustomWindowCommand {
  return {
    id: editingId ?? createCustomWindowCommandId(),
    title,
    ...dimensions,
  };
}

export interface CustomCommandFormState {
  editingId?: string;
  values?: Record<string, ExtensionFormValue>;
  errors?: CustomCommandFormErrors;
  notice?: string;
}

export type CustomCommandFormErrors = ExactDimensionErrors & { title?: string };

export const DIMENSION_LABELS: Record<ExactDimensionKey, string> = {
  x: 'X offset',
  y: 'Y offset',
  width: 'Width',
  height: 'Height',
};

export function unitFieldIdFor(key: ExactDimensionKey) {
  return `${key}Unit`;
}

export function customCommandFormValues(
  command?: CustomWindowCommand,
): Record<string, ExtensionFormValue> {
  if (!command) {
    return {};
  }
  const values: Record<string, ExtensionFormValue> = {
    [TITLE_FIELD_ID]: command.title,
  };
  for (const key of EXACT_DIMENSION_KEYS) {
    const dimension = command[key];
    if (!dimension) {
      continue;
    }
    values[key] = String(dimension.value);
    values[unitFieldIdFor(key)] = dimension.unit;
  }
  return values;
}

export function customCommandFormFields(
  values: Record<string, ExtensionFormValue>,
  errors: CustomCommandFormErrors,
): ExtensionFormField[] {
  return [
    {
      id: TITLE_FIELD_ID,
      label: 'Title',
      type: 'text',
      value: String(values[TITLE_FIELD_ID] ?? ''),
      placeholder: 'Left third, full height',
      required: true,
      error: errors.title,
    },
    ...EXACT_DIMENSION_KEYS.flatMap((key) =>
      dimensionFieldsFor(key, values, errors),
    ),
  ];
}

export function customWindowCommandFormView(
  ctx: ExtensionContext,
  store: CustomWindowCommandsStore,
  state: CustomCommandFormState = {},
): ExtensionView {
  const { editingId, values = {}, errors = {}, notice } = state;
  return ctx.ui.form({
    id: 'window-management-custom-command-form',
    title: editingId
      ? 'Edit Custom Window Command'
      : 'Create Custom Window Command',
    subtitle: notice ?? FORM_HINT,
    fields: customCommandFormFields(values, errors),
    submitAction: ctx.actions.run('Save Command', (innerCtx, action) =>
      saveCustomWindowCommand(
        innerCtx,
        store,
        editingId,
        action.formValues ?? {},
      ),
    ),
  });
}
