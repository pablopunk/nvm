import {
  WINDOW_LAYOUT_SPECS,
  type WindowLayoutId,
} from '../electron/os/window-layout-catalog';
import type { WindowLayoutSpec } from '../electron/os/window-management-types';
import type { ExtensionWindowLayoutSpec } from '../resources/nevermind-extension-api';
import type { WindowManagementSettings } from './window-management-settings';

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

function scaledCenterWithPercent(
  spec: ScaledCenterLayoutSpec,
  percent: number,
): ScaledCenterLayoutSpec {
  const fraction = percentToFraction(percent);
  return { ...spec, widthFraction: fraction, heightFraction: fraction };
}

function resizeStepWithPercent(
  spec: ResizeStepLayoutSpec,
  percent: number,
): ResizeStepLayoutSpec {
  return { ...spec, step: percentToFraction(percent) };
}

function buildAlmostMaximizeSpec(settings: WindowManagementSettings) {
  return scaledCenterWithPercent(
    WINDOW_LAYOUT_SPECS['almost-maximize'],
    settings.almostMaximizePercent,
  );
}

function buildReasonableSizeSpec(settings: WindowManagementSettings) {
  return scaledCenterWithPercent(
    WINDOW_LAYOUT_SPECS['reasonable-size'],
    settings.reasonableSizePercent,
  );
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
