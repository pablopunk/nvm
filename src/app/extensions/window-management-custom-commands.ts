import type {
  ExtensionContext,
  ExtensionItem,
  ExtensionView,
} from '../resources/nevermind-extension-api';
import {
  applyWindowSpecWithSettings,
  WINDOW_MANAGEMENT_OS_CAPABILITY,
  WINDOW_MANAGEMENT_SUBTITLE,
} from './window-management-commands';
import {
  customCommandFormValues,
  customWindowCommandFormView,
  DIMENSION_LABELS,
} from './window-management-custom-form';
import {
  type CustomWindowCommand,
  type CustomWindowCommandsStore,
  customWindowCommandDimensions,
  extensionWindowCustomCommandsStore,
  MAX_CUSTOM_WINDOW_COMMANDS,
} from './window-management-custom-store';
import {
  EXACT_DIMENSION_KEYS,
  exactWindowLayoutSpec,
  formatExactDimension,
} from './window-management-exact-values';

const LIST_VIEW_ID = 'window-management-custom-commands';

const ROW_ICON = 'layout-list';

const CREATE_ROW_ID = 'create-custom-command';

function describeDimensionPart(command: CustomWindowCommand) {
  return EXACT_DIMENSION_KEYS.flatMap((key) => {
    const dimension = command[key];
    return dimension
      ? [`${DIMENSION_LABELS[key]} ${formatExactDimension(dimension)}`]
      : [];
  });
}

function editCustomCommandAction(
  ctx: ExtensionContext,
  store: CustomWindowCommandsStore,
  command: CustomWindowCommand,
) {
  return ctx.actions.run('Edit Command', (innerCtx) =>
    customWindowCommandFormView(innerCtx, store, {
      editingId: command.id,
      values: customCommandFormValues(command),
    }),
  );
}

function deleteCustomCommandAction(
  ctx: ExtensionContext,
  store: CustomWindowCommandsStore,
  command: CustomWindowCommand,
) {
  return ctx.actions.run(
    'Delete Command',
    async (innerCtx) => {
      const commands = await store.load();
      await store.save(commands.filter((entry) => entry.id !== command.id));
      innerCtx.views.invalidate();
      return {
        patch: { items: await customWindowCommandItems(innerCtx, store) },
      };
    },
    {
      style: 'destructive',
      requiresConfirmation: true,
      confirmLabel: 'Delete',
      confirmMessage: `Delete “${command.title}”? This cannot be undone.`,
    },
  );
}

function customCommandRow(
  ctx: ExtensionContext,
  store: CustomWindowCommandsStore,
  command: CustomWindowCommand,
): ExtensionItem {
  return ctx.ui.item({
    id: command.id,
    title: command.title,
    subtitle: describeCustomWindowCommand(command),
    icon: ROW_ICON,
    primaryAction: runCustomWindowCommandAction(ctx, command),
    actionPanel: {
      sections: [
        {
          actions: [
            editCustomCommandAction(ctx, store, command),
            deleteCustomCommandAction(ctx, store, command),
          ],
        },
      ],
    },
  });
}

function createCustomCommandRow(
  ctx: ExtensionContext,
  store: CustomWindowCommandsStore,
): ExtensionItem {
  return ctx.ui.item({
    id: CREATE_ROW_ID,
    title: 'Create Custom Command…',
    subtitle: `Up to ${MAX_CUSTOM_WINDOW_COMMANDS} commands`,
    icon: 'plus',
    primaryAction: ctx.actions.run('Create Custom Command', (innerCtx) =>
      customWindowCommandFormView(innerCtx, store),
    ),
  });
}

function matchesQuery(command: CustomWindowCommand, needle: string) {
  return command.title.toLowerCase().includes(needle);
}

export const CUSTOM_COMMANDS_TITLE = 'Custom Window Commands';

export function describeCustomWindowCommand(command: CustomWindowCommand) {
  return describeDimensionPart(command).join(' · ');
}

export function runCustomWindowCommandAction(
  ctx: ExtensionContext,
  command: CustomWindowCommand,
) {
  return ctx.actions.run(
    command.title,
    (innerCtx) =>
      applyWindowSpecWithSettings(innerCtx, command.title, () =>
        exactWindowLayoutSpec(customWindowCommandDimensions(command)),
      ),
    { dismissAfterRun: 'auto' },
  );
}

export async function customWindowCommandItems(
  ctx: ExtensionContext,
  store: CustomWindowCommandsStore,
): Promise<ExtensionItem[]> {
  const commands = await store.load();
  return [
    createCustomCommandRow(ctx, store),
    ...commands.map((command) => customCommandRow(ctx, store, command)),
  ];
}

export async function customWindowCommandsView(
  ctx: ExtensionContext,
  store: CustomWindowCommandsStore = extensionWindowCustomCommandsStore(ctx),
): Promise<ExtensionView> {
  return ctx.ui.list({
    id: LIST_VIEW_ID,
    title: CUSTOM_COMMANDS_TITLE,
    subtitle: 'Exact sizes and offsets for your windows',
    presentation: 'root',
    searchBarPlaceholder: 'Search custom window commands',
    items: await customWindowCommandItems(ctx, store),
  });
}

export function customWindowCommandsCommand() {
  return {
    id: 'custom-commands',
    title: CUSTOM_COMMANDS_TITLE,
    subtitle: WINDOW_MANAGEMENT_SUBTITLE,
    icon: ROW_ICON,
    run: (ctx: ExtensionContext) => customWindowCommandsView(ctx),
  };
}

export async function customWindowCommandSearchItems(
  ctx: ExtensionContext,
  query: string,
): Promise<ExtensionItem[]> {
  const needle = query.trim().toLowerCase();
  if (!needle) {
    return [];
  }
  if (!ctx.system.capabilities.has(WINDOW_MANAGEMENT_OS_CAPABILITY)) {
    return [];
  }
  const commands = await extensionWindowCustomCommandsStore(ctx).load();
  return commands
    .filter((command) => matchesQuery(command, needle))
    .map((command) =>
      ctx.ui.item({
        id: `custom:${command.id}`,
        title: command.title,
        subtitle: describeCustomWindowCommand(command),
        icon: ROW_ICON,
        primaryAction: runCustomWindowCommandAction(ctx, command),
      }),
    );
}
