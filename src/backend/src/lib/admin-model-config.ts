import { listAdminModelRefs } from './admin-model-options';
import { dashboardRead } from './dashboard-read';
import { isSelectableDecisionModel, listDecisionModels } from './decisions/decision-models';
import { routeKindForSlot, type RouteKind } from './pricing';
import {
  DEFAULT_THINKING_LEVEL,
  getAudioModelRoute,
  getDecisionModelRoute,
  getModelRoute,
  isProviderEnabled,
  ModelNotConfiguredError,
  modelRouteToRef,
  type ModelRouteSlot,
  THINKING_LEVELS,
  type ThinkingLevel,
} from './settings';
import { listSelectableTranscriptionModels } from './transcription-eligibility';

export type AdminModelOption = { value: string; label: string; disabled?: boolean };
export type AdminTextRoute = { slot: ModelRouteSlot; title: string; kind: RouteKind; ref: string | null; thinkingLevel: ThinkingLevel; options: AdminModelOption[] };
export type AdminSingleRoute = { ref: string | null; options: AdminModelOption[] };
export type AdminModelConfig = { textRoutes: AdminTextRoute[]; dictation: AdminSingleRoute; decisions: AdminSingleRoute; thinkingLevels: readonly ThinkingLevel[] };

const TEXT_ROUTE_TITLES = [
  ['pro-smart', 'Pro smart'],
  ['pro-fast', 'Pro fast'],
  ['free-smart', 'Free smart'],
  ['free-fast', 'Free fast'],
] as const;

function nullWhenNotConfigured<T>(read: Promise<T>): Promise<T | null> {
  return read.catch((error) => {
    if (error instanceof ModelNotConfiguredError) return null;
    throw error;
  });
}

function optionsKeepingCurrent(current: string | null, selectable: AdminModelOption[], unavailableLabel: string, placeholder?: string): AdminModelOption[] {
  const selectableValues = new Set(selectable.map((option) => option.value));
  return [
    ...(!current && placeholder ? [{ value: '', label: placeholder, disabled: true }] : []),
    ...(current && !selectableValues.has(current) ? [{ value: current, label: `${current} (${unavailableLabel})`, disabled: true }] : []),
    ...selectable,
  ];
}

function refOptions(refs: string[]): AdminModelOption[] {
  return refs.map((ref) => ({ value: ref, label: ref }));
}

async function readTextRoutes(): Promise<AdminTextRoute[]> {
  const [smartRefs, fastRefs] = await Promise.all([listAdminModelRefs('smart'), listAdminModelRefs('fast')]);
  return Promise.all(TEXT_ROUTE_TITLES.map(async ([slot, title]) => {
    const route = await nullWhenNotConfigured(getModelRoute(slot));
    const ref = route ? modelRouteToRef(route) : null;
    const kind = routeKindForSlot(slot);
    return {
      slot,
      title,
      kind,
      ref,
      thinkingLevel: route?.thinkingLevel ?? DEFAULT_THINKING_LEVEL,
      options: optionsKeepingCurrent(ref, refOptions(kind === 'smart' ? smartRefs : fastRefs), 'not selectable'),
    };
  }));
}

async function readDictationRoute(): Promise<AdminSingleRoute> {
  const route = await nullWhenNotConfigured(getAudioModelRoute());
  const ref = route ? modelRouteToRef(route) : null;
  const refs = (await isProviderEnabled('openrouter')) ? (await listSelectableTranscriptionModels()).map(modelRouteToRef) : [];
  return { ref, options: optionsKeepingCurrent(ref, refOptions(refs), 'provider, pricing, or budget unavailable', 'Select a dictation model') };
}

async function readDecisionRoute(): Promise<AdminSingleRoute> {
  const route = await nullWhenNotConfigured(getDecisionModelRoute());
  const ref = route ? modelRouteToRef(route) : null;
  const refs = (await listDecisionModels()).filter(isSelectableDecisionModel).map(modelRouteToRef);
  return { ref, options: optionsKeepingCurrent(ref, refOptions(refs), 'not eligible', 'Select a decision model') };
}

export async function readAdminModelConfig(): Promise<AdminModelConfig> {
  const [textRoutes, dictation, decisions] = await dashboardRead(() => Promise.all([readTextRoutes(), readDictationRoute(), readDecisionRoute()]));
  return { textRoutes, dictation, decisions, thinkingLevels: THINKING_LEVELS };
}
