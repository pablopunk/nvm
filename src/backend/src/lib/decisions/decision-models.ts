import type { ModelCost } from '../cost';
import {
  durableOpenRouterCatalog,
  openRouterCatalogData,
} from '../openrouter-catalog-snapshots';
import { staleAdmissionsHalted } from '../pricing-overrun-cap';

export type DecisionModel = {
  provider: 'openrouter';
  modelId: string;
  name: string;
  acceptsImages: boolean;
  cost?: ModelCost | null;
  contextLength?: number;
  imageTokenPriced?: boolean;
};

type OpenRouterModel = {
  id?: unknown;
  name?: unknown;
  pricing?: {
    prompt?: unknown;
    completion?: unknown;
    image?: unknown;
    request?: unknown;
  };
  context_length?: unknown;
  architecture?: {
    input_modalities?: unknown;
    output_modalities?: unknown;
  };
};

function hasModality(value: unknown, modality: string): boolean {
  return Array.isArray(value) && value.includes(modality);
}

export function decisionModelFromCatalog(
  model: OpenRouterModel,
): DecisionModel | null {
  if (
    typeof model.id !== 'string' ||
    !hasModality(model.architecture?.output_modalities, 'decisions')
  )
    return null;
  return {
    provider: 'openrouter',
    modelId: model.id,
    name: typeof model.name === 'string' ? model.name : model.id,
    acceptsImages: hasModality(model.architecture?.input_modalities, 'image'),
    cost: decisionCatalogCost(model),
    ...(typeof model.context_length === 'number' &&
    Number.isSafeInteger(model.context_length) &&
    model.context_length > 0
      ? { contextLength: model.context_length }
      : {}),
    imageTokenPriced:
      (isZeroPrice(model.pricing?.image) ||
        (model.id === 'typesafe/jev-1.14' &&
          model.pricing?.image === undefined)) &&
      (model.pricing?.request === undefined ||
        isZeroPrice(model.pricing.request)),
  };
}

function isZeroPrice(value: unknown): boolean {
  return (
    typeof value === 'string' && value.trim() !== '' && Number(value) === 0
  );
}

/** New routes must support images and have the billing bounds used by execution. */
export function isSelectableDecisionModel(model: DecisionModel): boolean {
  return Boolean(
    model.acceptsImages &&
      model.contextLength &&
      model.imageTokenPriced &&
      model.cost &&
      model.cost.outputUsdPerMtok === 0,
  );
}

function decisionCatalogCost(model: OpenRouterModel): ModelCost | null {
  const input = model.pricing?.prompt;
  const output = model.pricing?.completion;
  if (
    typeof model.id !== 'string' ||
    typeof input !== 'string' ||
    typeof output !== 'string' ||
    !input.trim() ||
    !output.trim()
  )
    return null;
  const inputRate = Number(input);
  const outputRate = Number(output);
  if (
    !Number.isFinite(inputRate) ||
    inputRate < 0 ||
    !Number.isFinite(outputRate) ||
    outputRate < 0
  )
    return null;
  return {
    provider: 'openrouter',
    modelId: model.id,
    inputUsdPerMtok: inputRate * 1_000_000,
    outputUsdPerMtok: outputRate * 1_000_000,
  };
}

function parseDecisionCatalog(body: unknown): DecisionModel[] | null {
  const data = openRouterCatalogData(body);
  if (!data) return null;
  return (data as OpenRouterModel[])
    .map(decisionModelFromCatalog)
    .filter((model): model is DecisionModel => Boolean(model))
    .sort((left, right) => left.name.localeCompare(right.name));
}

function isDecisionModelList(payload: unknown): payload is DecisionModel[] {
  return (
    Array.isArray(payload) &&
    payload.every(
      (model) =>
        typeof model === 'object' &&
        model !== null &&
        typeof (model as DecisionModel).modelId === 'string' &&
        typeof (model as DecisionModel).acceptsImages === 'boolean',
    )
  );
}

const decisionCatalog = durableOpenRouterCatalog<DecisionModel>({
  snapshotKey: 'openrouter_decision_models',
  path: '/models?output_modalities=decisions',
  parseResponse: parseDecisionCatalog,
  isPayload: isDecisionModelList,
});

/** Validates the decision catalog. Only the scheduled refresh and admin saves fetch it. */
export function refreshDecisionCatalog(): Promise<void> {
  return decisionCatalog.refresh();
}

/** Listed models come from the admissible catalog: stale still lists, unvalidated or expired lists nothing. */
export async function listDecisionModels(): Promise<DecisionModel[]> {
  return (await decisionCatalog.admissible())?.value ?? [];
}

/** The eligible model, its cost marked stale past the current window. Never fetches. */
export async function findEligibleDecisionModel(
  provider: string,
  modelId: string,
): Promise<DecisionModel | null> {
  if (provider !== 'openrouter') return null;
  const catalog = await decisionCatalog.admissible();
  const model = catalog?.value.find(
    (candidate) => candidate.provider === provider && candidate.modelId === modelId,
  );
  if (!model) return null;
  if (catalog?.validity !== 'stale' || !model.cost) return model;
  if (await staleAdmissionsHalted()) return null;
  return { ...model, cost: { ...model.cost, stale: true } };
}

export async function lookupDecisionModelCost(
  provider: string,
  modelId: string,
): Promise<ModelCost | null> {
  return (await findEligibleDecisionModel(provider, modelId))?.cost ?? null;
}

export function resetDecisionModelCacheForTests() {
  decisionCatalog.reset();
}
