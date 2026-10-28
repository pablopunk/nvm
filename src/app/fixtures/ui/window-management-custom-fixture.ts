import { customWindowCommandsView } from '../../extensions/window-management-custom-commands';
import { customWindowCommandFormView } from '../../extensions/window-management-custom-form';
import {
  type CustomWindowCommand,
  type CustomWindowCommandsStore,
  normalizeCustomWindowCommands,
} from '../../extensions/window-management-custom-store';
import type { ExtensionContext } from '../../resources/nevermind-extension-api';

const SAMPLE_COMMANDS: CustomWindowCommand[] = [
  {
    id: 'fixture-left-third',
    title: 'Left third',
    x: { value: 0, unit: 'points' },
    width: { value: 33, unit: 'percent' },
    height: { value: 100, unit: 'percent' },
  },
  {
    id: 'fixture-centered',
    title: 'Centered 800 × 600',
    width: { value: 800, unit: 'points' },
    height: { value: 600, unit: 'points' },
  },
];

function memoryCustomWindowCommandsStore(
  seed: CustomWindowCommand[],
): CustomWindowCommandsStore {
  let commands = normalizeCustomWindowCommands(seed);
  return {
    load: () => Promise.resolve(commands),
    save(next) {
      commands = normalizeCustomWindowCommands(next);
      return Promise.resolve();
    },
  };
}

const fixtureStore = memoryCustomWindowCommandsStore(SAMPLE_COMMANDS);

export function windowManagementCustomCommandsFixtureView(
  ctx: ExtensionContext,
) {
  return customWindowCommandsView(ctx, fixtureStore);
}

export function windowManagementCustomFormErrorFixtureView(
  ctx: ExtensionContext,
) {
  return customWindowCommandFormView(ctx, fixtureStore, {
    values: {
      title: '',
      x: '120',
      xUnit: 'percent',
      width: '0',
      widthUnit: 'percent',
    },
    errors: {
      title: 'Enter a title.',
      x: 'Enter a number from 0 to 100%.',
      width: 'Enter a number above 0, up to 100%.',
    },
  });
}
