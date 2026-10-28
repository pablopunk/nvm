import {
  MAX_WINDOW_GAP_POINTS,
  type WindowEdge,
} from '../electron/os/window-management-types';
import type { WindowManagementSettingKey } from './window-management-settings-model';

const GAP_MAX = MAX_WINDOW_GAP_POINTS;

function wholeNumberSetting(
  definition: Omit<WholeNumberSettingDefinition, 'kind' | 'nullable'> &
    Partial<Pick<WholeNumberSettingDefinition, 'nullable'>>,
): WholeNumberSettingDefinition {
  return { kind: 'whole-number', nullable: false, ...definition };
}

function booleanSetting(
  definition: Omit<BooleanSettingDefinition, 'kind'>,
): BooleanSettingDefinition {
  return { kind: 'boolean', ...definition };
}

function choiceSetting(
  definition: Omit<ChoiceSettingDefinition, 'kind'>,
): ChoiceSettingDefinition {
  return { kind: 'choice', ...definition };
}

function edgeGapSetting(
  key: WindowManagementSettingKey,
  edge: WindowEdge,
): WholeNumberSettingDefinition {
  return wholeNumberSetting({
    key,
    title: `${edge[0].toUpperCase()}${edge.slice(1)} Gap`,
    description: `Overrides the outer gap on the ${edge} edge. Leave blank to follow Outer Gap.`,
    unit: 'px',
    min: 0,
    max: GAP_MAX,
    nullable: true,
  });
}

const OUTER_GAP_SETTING = wholeNumberSetting({
  key: 'outerGap',
  title: 'Outer Gap',
  description: 'Space kept between windows and every display edge.',
  unit: 'px',
  min: 0,
  max: GAP_MAX,
});

const GAP_SECTION: WindowManagementSettingSection = {
  id: 'gaps',
  title: 'Gaps',
  subtitle: 'Space around and between tiled windows',
  settings: [
    OUTER_GAP_SETTING,
    edgeGapSetting('outerGapTop', 'top'),
    edgeGapSetting('outerGapRight', 'right'),
    edgeGapSetting('outerGapBottom', 'bottom'),
    edgeGapSetting('outerGapLeft', 'left'),
    wholeNumberSetting({
      key: 'innerGap',
      title: 'Inner Gap',
      description: 'Space between two adjacent tiled windows.',
      unit: 'px',
      min: 0,
      max: GAP_MAX,
    }),
    booleanSetting({
      key: 'applyGapToMaximize',
      title: 'Gap on Maximize',
      description: 'Keep the outer gap when Maximize fills the display.',
    }),
  ],
};

const SIZE_SECTION: WindowManagementSettingSection = {
  id: 'sizes',
  title: 'Sizes',
  subtitle: 'Almost Maximize, Reasonable Size, Center Half, and resize steps',
  settings: [
    wholeNumberSetting({
      key: 'almostMaximizeWidthPercent',
      title: 'Almost Maximize Width',
      description: 'Share of the work area width that Almost Maximize fills.',
      unit: '%',
      min: 50,
      max: 100,
    }),
    wholeNumberSetting({
      key: 'almostMaximizeHeightPercent',
      title: 'Almost Maximize Height',
      description: 'Share of the work area height that Almost Maximize fills.',
      unit: '%',
      min: 50,
      max: 100,
    }),
    wholeNumberSetting({
      key: 'reasonableSizeWidthPercent',
      title: 'Reasonable Size Width',
      description: 'Share of the work area width that Reasonable Size fills.',
      unit: '%',
      min: 30,
      max: 100,
    }),
    wholeNumberSetting({
      key: 'reasonableSizeHeightPercent',
      title: 'Reasonable Size Height',
      description: 'Share of the work area height that Reasonable Size fills.',
      unit: '%',
      min: 30,
      max: 100,
    }),
    wholeNumberSetting({
      key: 'centerHalfWidthPercent',
      title: 'Center Half Width',
      description: 'Share of the work area width that Center Half fills.',
      unit: '%',
      min: 20,
      max: 90,
    }),
    wholeNumberSetting({
      key: 'resizeStepPercent',
      title: 'Resize Step',
      description:
        'Share of the work area that Make Larger and Make Smaller change by.',
      unit: '%',
      min: 1,
      max: 25,
    }),
  ],
};

const BEHAVIOR_SECTION: WindowManagementSettingSection = {
  id: 'behavior',
  title: 'Behavior',
  subtitle: 'Repeated half commands',
  settings: [
    booleanSetting({
      key: 'cycle',
      title: 'Cycle Half Commands',
      description: 'Repeat a half command on the same window to cycle sizes.',
    }),
    choiceSetting({
      key: 'cycleOrder',
      title: 'Cycle Order',
      description: 'Sizes that repeated half commands cycle through.',
      options: [
        { value: 'half-two-thirds-third', label: '½ → ⅔ → ⅓' },
        { value: 'half-third-two-thirds', label: '½ → ⅓ → ⅔' },
      ],
    }),
  ],
};

const DISPLAY_SECTION: WindowManagementSettingSection = {
  id: 'displays',
  title: 'Displays',
  subtitle: 'Next and Previous Display',
  settings: [
    choiceSetting({
      key: 'displayOrder',
      title: 'Display Order',
      description:
        'Order that Next and Previous Display walk through displays.',
      options: [
        { value: 'left-to-right', label: 'Left to right' },
        { value: 'top-to-bottom', label: 'Top to bottom' },
      ],
    }),
    choiceSetting({
      key: 'displayMoveSizing',
      title: 'Display Move Sizing',
      description: 'How a window is sized when it moves to another display.',
      options: [
        { value: 'proportional', label: 'Proportional' },
        { value: 'keep-size', label: 'Keep size' },
      ],
    }),
  ],
};

export const WINDOW_MANAGEMENT_SETTING_SECTIONS: WindowManagementSettingSection[] =
  [GAP_SECTION, SIZE_SECTION, BEHAVIOR_SECTION, DISPLAY_SECTION];

export const WINDOW_MANAGEMENT_SETTING_DEFINITIONS: WindowManagementSettingDefinition[] =
  WINDOW_MANAGEMENT_SETTING_SECTIONS.flatMap((section) => section.settings);

export function wholeNumberSettingFor(key: WindowManagementSettingKey) {
  const definition = WINDOW_MANAGEMENT_SETTING_DEFINITIONS.find(
    (candidate) => candidate.key === key,
  );
  if (definition?.kind === 'whole-number') {
    return definition;
  }
  throw new Error(`expected whole-number setting ${key}`);
}

export interface WholeNumberSettingDefinition {
  kind: 'whole-number';
  key: WindowManagementSettingKey;
  title: string;
  description: string;
  unit: 'px' | '%';
  min: number;
  max: number;
  nullable: boolean;
}

export interface BooleanSettingDefinition {
  kind: 'boolean';
  key: WindowManagementSettingKey;
  title: string;
  description: string;
}

export interface ChoiceOption {
  value: string;
  label: string;
}

export interface ChoiceSettingDefinition {
  kind: 'choice';
  key: WindowManagementSettingKey;
  title: string;
  description: string;
  options: ChoiceOption[];
}

export type WindowManagementSettingDefinition =
  | WholeNumberSettingDefinition
  | BooleanSettingDefinition
  | ChoiceSettingDefinition;

export interface WindowManagementSettingSection {
  id: string;
  title: string;
  subtitle: string;
  settings: WindowManagementSettingDefinition[];
}
