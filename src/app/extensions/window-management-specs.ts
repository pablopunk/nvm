import {
  WINDOW_LAYOUT_SPECS,
  type WindowLayoutId,
} from '../electron/os/window-layout-catalog';
import type { WindowLayoutSpec } from '../electron/os/window-management-types';
import type { ExtensionWindowLayoutSpec } from '../resources/nevermind-extension-api';
import type { WindowManagementSettings } from './window-management-settings-model';

type ScaledCenterLayoutSpec = Extract<
  WindowLayoutSpec,
  { kind: 'scaled-center' }
>;
type ResizeStepLayoutSpec = Extract<WindowLayoutSpec, { kind: 'resize-step' }>;
type SettingsSpecBuilder = (
  settings: WindowManagementSettings,
) => ExtensionWindowLayoutSpec;

const PERCENT_PER_WHOLE = 100;

function percentToFraction(percent: number) {
  return percent / PERCENT_PER_WHOLE;
}

function scaledCenterWithPercents(
  spec: ScaledCenterLayoutSpec,
  widthPercent: number,
  heightPercent: number,
): ScaledCenterLayoutSpec {
  return {
    ...spec,
    widthFraction: percentToFraction(widthPercent),
    heightFraction: percentToFraction(heightPercent),
  };
}

function resizeStepWithPercent(
  spec: ResizeStepLayoutSpec,
  percent: number,
): ResizeStepLayoutSpec {
  return { ...spec, step: percentToFraction(percent) };
}

function buildAlmostMaximizeSpec(settings: WindowManagementSettings) {
  return scaledCenterWithPercents(
    WINDOW_LAYOUT_SPECS['almost-maximize'],
    settings.almostMaximizeWidthPercent,
    settings.almostMaximizeHeightPercent,
  );
}

function buildReasonableSizeSpec(settings: WindowManagementSettings) {
  return scaledCenterWithPercents(
    WINDOW_LAYOUT_SPECS['reasonable-size'],
    settings.reasonableSizeWidthPercent,
    settings.reasonableSizeHeightPercent,
  );
}

function buildCenterHalfSpec(
  settings: WindowManagementSettings,
): ExtensionWindowLayoutSpec {
  const width = percentToFraction(settings.centerHalfWidthPercent);
  return { kind: 'fraction', x: (1 - width) / 2, y: 0, width, height: 1 };
}

function buildMakeLargerSpec(settings: WindowManagementSettings) {
  return resizeStepWithPercent(
    WINDOW_LAYOUT_SPECS['make-larger'],
    settings.resizeStepPercent,
  );
}

function buildMakeSmallerSpec(settings: WindowManagementSettings) {
  return resizeStepWithPercent(
    WINDOW_LAYOUT_SPECS['make-smaller'],
    settings.resizeStepPercent,
  );
}

const SETTINGS_DEPENDENT_SPEC_BUILDERS: Partial<
  Record<WindowLayoutId, SettingsSpecBuilder>
> = {
  'almost-maximize': buildAlmostMaximizeSpec,
  'reasonable-size': buildReasonableSizeSpec,
  'center-half': buildCenterHalfSpec,
  'make-larger': buildMakeLargerSpec,
  'make-smaller': buildMakeSmallerSpec,
};

export function windowLayoutSpecFor(
  id: WindowLayoutId,
  settings: WindowManagementSettings,
): ExtensionWindowLayoutSpec {
  const buildSettingsSpec = SETTINGS_DEPENDENT_SPEC_BUILDERS[id];
  return buildSettingsSpec
    ? buildSettingsSpec(settings)
    : WINDOW_LAYOUT_SPECS[id];
}
