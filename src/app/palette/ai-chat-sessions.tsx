import { useLayoutEffect, useRef, useState } from 'react';
import { startNestedChatFixtureStream } from '../fixtures/nested-chat-stream';
import { AiChatEventOwners } from './ai-chat-event-routing';
import type { CommandView } from './model';
import type { AiChatEvent } from './preload-api';
import { useAiChat } from './use-ai-chat';

type Controller = ReturnType<typeof useAiChat>;
export function chatSessionKey(view: CommandView) {
  return view.chatId || `view:${view.id || view.title}`;
}

export function useAiChatSessions(
  views: CommandView[],
  activeView: CommandView | null,
) {
  const fallback = useAiChat(
    window.nvm.sendAiMessage,
    window.nvm.setAiChatModel,
    window.nvm.resetAiChat,
    () => false,
  );
  const controllers = useRef(new Map<string, Controller>());
  const pending = useRef(new Map<string, CommandView>());
  const activeKey = useRef<string | undefined>(undefined);
  const eventOwners = useRef(new AiChatEventOwners());
  const fallbackRef = useRef(fallback);
  fallbackRef.current = fallback;
  const [, refresh] = useState(0);
  const chatViews = [
    ...new Map(
      views
        .filter((view) => view.aiChat)
        .map((view) => [chatSessionKey(view), view]),
    ).values(),
  ];
  const visibleChat = activeView?.aiChat ? activeView : chatViews.at(-1);
  activeKey.current = visibleChat ? chatSessionKey(visibleChat) : undefined;
  const proxyRef = useRef<Controller | null>(null);
  if (!proxyRef.current)
    proxyRef.current = new Proxy({} as Controller, {
      get(_target, property) {
        if (property === 'openChat')
          return async (view: CommandView) => {
            const key = chatSessionKey(view);
            if (!controllers.current.has(key)) pending.current.set(key, view);
          };
        if (property === 'handleEvent')
          return (event: AiChatEvent) => {
            const key = eventOwners.current.sessionFor(event);
            const matching = key ? controllers.current.get(key) : undefined;
            matching?.handleEvent(event);
            eventOwners.current.finish(event);
          };
        if (property === 'sendPrompt')
          return async (message: string, chatId?: string) => {
            const key = chatId || activeKey.current;
            if (!key) return;
            await controllers.current.get(key)?.sendPrompt(message, chatId);
          };
        return Reflect.get(
          controllers.current.get(activeKey.current || '') ||
            fallbackRef.current,
          property,
        );
      },
    });
  function register(key: string, controller: Controller) {
    controllers.current.set(key, controller);
  }
  function release(key: string, controller: Controller) {
    if (controllers.current.get(key)?.inputRef !== controller.inputRef) return;
    const view = chatViews.find(
      (candidate) => chatSessionKey(candidate) === key,
    );
    if (view?.chatId) void window.nvm.aiChatExited(view.chatId);
    controllers.current.delete(key);
    pending.current.delete(key);
    eventOwners.current.release(key);
  }
  return {
    controller: proxyRef.current,
    sessionId: activeKey.current,
    sessionForEvent: (event: AiChatEvent) =>
      eventOwners.current.sessionFor(event),
    hasSession: (view: CommandView) =>
      controllers.current.has(chatSessionKey(view)),
    forView: (view: CommandView) =>
      controllers.current.get(chatSessionKey(view)) || fallback,
    providers: chatViews.map((view) => (
      <ChatSession
        key={chatSessionKey(view)}
        view={pending.current.get(chatSessionKey(view)) || view}
        active={Boolean(
          activeView?.aiChat &&
            chatSessionKey(activeView) === chatSessionKey(view),
        )}
        register={register}
        release={release}
        onSend={(key, traceId) => eventOwners.current.begin(key, traceId)}
        onSendFailure={(key, traceId) => eventOwners.current.fail(key, traceId)}
        notify={() => refresh((version) => version + 1)}
      />
    )),
  };
}

function ChatSession({
  view,
  active,
  register,
  release,
  notify,
  onSend,
  onSendFailure,
}: {
  view: CommandView;
  active: boolean;
  register: (key: string, controller: Controller) => void;
  release: (key: string, controller: Controller) => void;
  notify: () => void;
  onSend: (key: string, traceId?: string) => void;
  onSendFailure: (key: string, traceId?: string) => void;
}) {
  const activeRef = useRef(active);
  activeRef.current = active;
  const key = chatSessionKey(view);
  const fixture = import.meta.env.DEV && view.id === 'dev-ui-nested-ai-chat';
  const controllerRef = useRef<Controller | null>(null);
  const initialized = useRef(false);
  const mounted = useRef(false);
  const mountGeneration = useRef(0);
  const fixtureCleanup = useRef<(() => void) | undefined>(undefined);
  const controller = useAiChat(
    async (message, chatId, traceId, images) => {
      onSend(key, traceId);
      if (fixture) {
        fixtureCleanup.current?.();
        fixtureCleanup.current = startNestedChatFixtureStream(
          view.chatId!,
          (event) => controllerRef.current?.handleEvent(event),
        );
      } else {
        try {
          await window.nvm.sendAiMessage(message, chatId, traceId, images);
        } catch (error) {
          onSendFailure(key, traceId);
          throw error;
        }
      }
    },
    window.nvm.setAiChatModel,
    window.nvm.resetAiChat,
    () => activeRef.current,
  );
  controllerRef.current = controller;
  useLayoutEffect(() => {
    const generation = ++mountGeneration.current;
    mounted.current = true;
    register(key, controller);
    queueMicrotask(() => {
      if (
        !mounted.current ||
        mountGeneration.current !== generation ||
        initialized.current
      )
        return;
      initialized.current = true;
      void controller.openChat(view).then(() => {
        if (fixture && mounted.current)
          fixtureCleanup.current = startNestedChatFixtureStream(
            view.chatId!,
            (event) => controllerRef.current?.handleEvent(event),
          );
      });
    });
    return () => {
      mounted.current = false;
      queueMicrotask(() => {
        if (mountGeneration.current !== generation) return;
        fixtureCleanup.current?.();
        release(key, controller);
      });
    };
  }, [key]);
  useLayoutEffect(() => {
    register(key, controller);
    notify();
  }, [
    controller.messages,
    controller.input,
    controller.attachments,
    controller.busy,
    controller.activity,
    controller.model,
    controller.modelChanging,
    controller.limit,
    controller.creditNotice,
    controller.attaching,
    controller.attachmentError,
  ]);
  return null;
}
