// biome-ignore-all lint: View patches are dynamic host-side payloads.
export type OpenViewPatchDeps = {
  normalizeViewPatch: (patch: any, entry: any) => any;
  sendToPalette: (channel: string, ...args: unknown[]) => void;
};

export function createOpenViewPatcher(deps: OpenViewPatchDeps) {
  const { normalizeViewPatch, sendToPalette } = deps;

  function patchOpenView(viewId: string, patch: any) {
    const normalizedPatch = normalizeViewPatch(patch, null);
    structuredClone(normalizedPatch);
    sendToPalette('view:patch', {
      viewId,
      patch: normalizedPatch,
    });
  }

  return { patchOpenView };
}
