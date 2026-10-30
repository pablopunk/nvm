// biome-ignore-all lint: Ranked actions are dynamic extension payloads and user state is a loosely shaped persisted document.
import {
  actionTextSearchScore,
  effectiveLastUsed,
  priorityBoost,
  QUERY_FALLBACK_SEARCH_SCORE,
  recencyBoost,
} from './action-ranking';
import { normalize } from './search-utils';

export type ActionRankerDeps = {
  userStateStore: { get: () => Record<string, any> };
  scheduleSaveState: () => void;
};

export function createActionRanker(deps: ActionRankerDeps) {
  const { userStateStore, scheduleSaveState } = deps;

  function actionAliases(actionId: any) {
    const value = userStateStore.get().aliases[actionId];
    if (!value) return [];
    return Array.isArray(value) ? value : [value];
  }

  function actionSearchScore(action: any, query: any) {
    const q = normalize(query);
    if (!q) return action.score || 0;
    return actionTextSearchScore(action, q, actionAliases(action.id));
  }

  function usageBoost(actionId: any) {
    const count = userStateStore.get().recents[actionId]?.count || 0;
    return Math.min(90, count * 6);
  }

  function recentBoost(actionId: any) {
    const recent = userStateStore.get().recents[actionId];
    return recencyBoost(recent?.lastUsed);
  }

  function rankAction(action: any, query: any) {
    const base = actionSearchScore(action, query);
    if (query.trim() && base <= 0) return null;
    const lastUsed = effectiveLastUsed(
      action.lastUsed,
      userStateStore.get().recents[action.id]?.lastUsed,
    );
    const boost =
      priorityBoost(action) + usageBoost(action.id) + recencyBoost(lastUsed);
    return {
      ...action,
      aliases: [...(action.aliases || []), ...actionAliases(action.id)],
      userAliases: actionAliases(action.id),
      score:
        base +
        (action.queryFallback === true && base === QUERY_FALLBACK_SEARCH_SCORE
          ? Math.min(9, boost)
          : boost),
      lastUsed,
    };
  }

  function recordRecent(action: any) {
    if (!action?.id) return;
    const current = userStateStore.get().recents[action.id] || {
      count: 0,
      lastUsed: 0,
      title: action.title,
    };
    userStateStore.get().recents[action.id] = {
      count: current.count + 1,
      lastUsed: Date.now(),
      title: action.title,
      kind: action.kind,
    };
    scheduleSaveState();
  }

  return {
    actionAliases,
    actionSearchScore,
    usageBoost,
    recentBoost,
    rankAction,
    recordRecent,
  };
}
