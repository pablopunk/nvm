// biome-ignore-all lint: Moved verbatim from main.ts; keeps its established host-side conventions.
import {
  getDefaultNevermindBaseUrl,
  nevermindEnvironmentForBaseUrl,
} from './nevermind-auth';
import { currentNevermindCompatibilityManifest } from './nevermind-compatibility';

export type NevermindEnvironmentSelectionDeps = {
  getStoredEnvironment: () => any;
  getActiveBaseUrl: () => string | null;
};

export function createNevermindEnvironmentSelection(
  deps: NevermindEnvironmentSelectionDeps,
) {
  const { getStoredEnvironment, getActiveBaseUrl } = deps;

  function selectedNevermindEnvironment() {
    const selected = getStoredEnvironment();
    if (!selected?.baseUrl) {
      return {
        environment: nevermindEnvironmentForBaseUrl(
          getDefaultNevermindBaseUrl(),
        ),
        baseUrl: getDefaultNevermindBaseUrl(),
      };
    }
    return selected;
  }

  function getNevermindDebugStatus() {
    const client = selectedNevermindEnvironment();
    const baseUrl = getActiveBaseUrl() || client.baseUrl;
    const manifest = currentNevermindCompatibilityManifest(baseUrl);
    return {
      client,
      active: {
        environment: nevermindEnvironmentForBaseUrl(baseUrl),
        baseUrl,
      },
      backend:
        manifest?.backend?.environment && manifest.backend.version
          ? {
              environment: manifest.backend.environment,
              version: manifest.backend.version,
            }
          : null,
    };
  }

  return { selectedNevermindEnvironment, getNevermindDebugStatus };
}
