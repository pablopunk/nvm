import type { AiChatEvent } from '../palette/preload-api';

/** Feeds the real chat controller without credentials or a model request. */
export function startNestedChatFixtureStream(
  chatId: string,
  emit: (event: AiChatEvent) => void,
) {
  emit({ type: 'start', chatId });
  let tick = 0;
  const timer = setInterval(function emitFixtureDelta() {
    emit({ type: 'delta', chatId, text: ` ${++tick}` });
    if (tick === 40) {
      clearInterval(timer);
      emit({ type: 'done', chatId });
    }
  }, 500);
  return function stopFixtureStream() {
    clearInterval(timer);
  };
}
