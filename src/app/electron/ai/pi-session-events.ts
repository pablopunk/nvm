import type { AgentSessionEvent } from '@earendil-works/pi-coding-agent';
import type { AssistantMessageEvent } from '@earendil-works/pi-ai';

type TextDeltaMessageUpdateEvent = Extract<
  AgentSessionEvent,
  { type: 'message_update' }
> & {
  assistantMessageEvent: Extract<AssistantMessageEvent, { type: 'text_delta' }>;
};

export function isMessageUpdateEvent(
  event: AgentSessionEvent,
): event is TextDeltaMessageUpdateEvent {
  return (
    event.type === 'message_update' &&
    event.assistantMessageEvent.type === 'text_delta' &&
    typeof event.assistantMessageEvent.delta === 'string'
  );
}
