// biome-ignore-all lint: AI chat events are dynamic payloads forwarded to renderer windows.
export type AiChatEventBroadcasterDeps = {
  sendToPalette: (channel: string, event: unknown) => void;
  broadcastToExtensionWindows: (channel: string, event: unknown) => void;
};

export function createAiChatEventBroadcaster(deps: AiChatEventBroadcasterDeps) {
  const { sendToPalette, broadcastToExtensionWindows } = deps;

  function broadcastAiChatEvent(event) {
    sendToPalette('ai:chat:event', event);
    broadcastToExtensionWindows('ai:chat:event', event);
  }

  return { broadcastAiChatEvent };
}
