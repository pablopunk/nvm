import { observeProviderStream } from '../diagnostics/backend-observability';

type PiApi = typeof import('@earendil-works/pi-coding-agent');
type PiCompatibilityApi = typeof import('@earendil-works/pi-ai/compat');
type PiModelRuntime = Awaited<ReturnType<PiApi['ModelRuntime']['create']>>;
type PiProviderConfig = Parameters<PiModelRuntime['registerProvider']>[1];
let piCompatibilityApi: Promise<PiCompatibilityApi> | null = null;

export async function registerObservedPiProvider(
  modelRuntime: PiModelRuntime,
  providerId: string,
  config: PiProviderConfig,
) {
  piCompatibilityApi ??= import('@earendil-works/pi-ai/compat');
  const compatibility = await piCompatibilityApi;
  const nativeProvider = modelRuntime.getRegisteredNativeProvider(providerId);
  const nativeApis = new Set(
    nativeProvider?.getModels().map((model) => model.api) ?? [],
  );
  modelRuntime.registerProvider(providerId, {
    ...config,
    streamSimple: observeProviderStream(
      function streamPiProvider(model, context, options) {
        if (nativeProvider && nativeApis.has(model.api))
          return nativeProvider.streamSimple(model, context, options);
        const apiProvider = compatibility.getApiProvider(model.api);
        if (!apiProvider)
          throw new Error(
            `No Pi API adapter is registered for configured API "${model.api}".`,
          );
        return apiProvider.streamSimple(model, context, options);
      },
    ),
  });
}
