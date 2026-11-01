/**
 * The only facts about a visible view that ever leave the machine: a host-defined
 * category. Titles, queries, paths and extension identifiers are never part of it.
 */
export const APP_VIEWS = ['palette', 'extension', 'ai-chat'] as const;

export type AppView = (typeof APP_VIEWS)[number];

export function isAppView(value: unknown): value is AppView {
  return APP_VIEWS.some((view) => view === value);
}

// Desktop pageviews use synthetic URLs so PostHog path analysis works without
// exposing anything about what the view contains.
export const APP_VIEW_HOST = 'app.nevermind.invalid';

export function appViewUrl(view: AppView) {
  return `https://${APP_VIEW_HOST}/${view}`;
}
