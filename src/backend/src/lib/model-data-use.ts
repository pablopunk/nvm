const PROVIDER_TRAINS_ON_PROMPTS_MODEL_ID = /(^|[/-])contributor($|-)/;

export const PROVIDER_TRAINS_ON_PROMPTS_LABEL = 'Provider may train on prompts and outputs';

export function modelProviderMayTrainOnPrompts(modelRef: string) {
  return PROVIDER_TRAINS_ON_PROMPTS_MODEL_ID.test(modelRef);
}
