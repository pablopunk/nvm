import { type SetStateAction, useRef, useState } from 'react';

/** Builder navigation belongs to the retained chat, not the active tool pane. */
export function useSessionState<T>(sessionId: string | undefined, initial: T) {
  const values = useRef(new Map<string, T>());
  const currentSession = useRef(sessionId || 'root');
  currentSession.current = sessionId || 'root';
  const [, refresh] = useState(0);
  function setForSession(key: string, value: SetStateAction<T>) {
    const current = values.current.get(key) ?? initial;
    const next =
      typeof value === 'function'
        ? (value as (current: T) => T)(current)
        : value;
    values.current.set(key, next);
    refresh((version) => version + 1);
  }
  function setValue(value: SetStateAction<T>) {
    setForSession(currentSession.current, value);
  }
  return [
    values.current.get(currentSession.current) ?? initial,
    setValue,
    setForSession,
  ] as const;
}
