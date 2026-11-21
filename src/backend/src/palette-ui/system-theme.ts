import { useSyncExternalStore } from 'react';
import type { CommandImage } from './model';

type SystemTheme = 'light' | 'dark';

interface ThemeMediaQuery {
  matches: boolean;
  addEventListener(type: 'change', listener: () => void): void;
  removeEventListener(type: 'change', listener: () => void): void;
}

type ThemeMediaQueryFactory = () => ThemeMediaQuery | null;

function createSystemThemeStore(createMediaQuery: ThemeMediaQueryFactory) {
  let mediaQuery: ThemeMediaQuery | null | undefined;

  function query() {
    if (mediaQuery === undefined) {
      mediaQuery = createMediaQuery();
    }
    return mediaQuery;
  }

  function getSnapshot(): SystemTheme {
    return query()?.matches ? 'dark' : 'light';
  }

  function subscribe(listener: () => void) {
    const activeQuery = query();
    activeQuery?.addEventListener('change', listener);
    return function unsubscribe() {
      activeQuery?.removeEventListener('change', listener);
    };
  }

  return { getSnapshot, subscribe };
}

const systemThemeStore = createSystemThemeStore(function createMediaQuery() {
  if (typeof window === 'undefined' || !window.matchMedia) {
    return null;
  }
  return window.matchMedia('(prefers-color-scheme: dark)');
});

function useSystemTheme() {
  return useSyncExternalStore(
    systemThemeStore.subscribe,
    systemThemeStore.getSnapshot,
    systemThemeStore.getSnapshot,
  );
}

function themedImageSource(
  image: CommandImage | undefined,
  theme: SystemTheme,
) {
  if (!image) {
    return '';
  }
  if (typeof image === 'string') {
    return image;
  }
  const themedSource = theme === 'dark' ? image.dark : image.light;
  const alternateSource = theme === 'dark' ? image.light : image.dark;
  return themedSource || image.src || alternateSource || image.fallback || '';
}

export type { SystemTheme };
export { createSystemThemeStore, themedImageSource, useSystemTheme };
