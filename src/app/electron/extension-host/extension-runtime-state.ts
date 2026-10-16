// biome-ignore-all lint: Host-side extension runtime keeps dynamic extension shapes by design.
import type { JobDefinition } from '../platform/jobs';

export type PreparedFileWatcher = {
  extensionId: string;
  event: string;
  trigger: any;
};

export type ExtensionFileWatcher = PreparedFileWatcher & {
  close: () => unknown;
};

export type ExtensionOwnedRecord = { extensionId?: string };

export type PreparedExtensionRuntime = {
  filePath: string;
  extensionIds: Set<string>;
  modules: Map<string, any>;
  actions: Map<string, any>;
  handlers: Map<string, any>;
  viewActions: Map<string, any>;
  rootActions: Map<string, any>;
  viewRefreshes: Map<string, any>;
  jobs: JobDefinition[];
  fileWatchers: PreparedFileWatcher[];
};

export type LiveExtensionRuntimeSnapshot = {
  extensionIds: Set<string>;
  modules: Map<string, any>;
  actions: Map<string, any>;
  handlers: Map<string, any>;
  viewActions: Map<string, ExtensionOwnedRecord>;
  rootActions: Map<string, ExtensionOwnedRecord>;
  viewRefreshes: Map<string, any>;
  jobs: JobDefinition[];
  fileWatchers: PreparedFileWatcher[];
};

export type ExtensionRuntimeRegistries = {
  extensionModules: Map<string, any>;
  extensionActionRegistry: Map<string, any>;
  extensionActionHandlers: Map<string, any>;
  viewActionExecutionRecords: Map<string, ExtensionOwnedRecord>;
  rootActionExecutionRecords: Map<string, ExtensionOwnedRecord>;
  viewRefreshRecords: Map<string, any>;
  extensionRootItemsCache: Map<string, any>;
  extensionRootItemsRefreshes: Map<string, Promise<any[]>>;
};

export type ExtensionRuntimeState = {
  extensionFileWatchers: ExtensionFileWatcher[];
  extensionRuntimePreparation:
    | Pick<PreparedExtensionRuntime, 'jobs' | 'fileWatchers'>
    | undefined;
  testExtensionActivationFailurePhase: string | undefined;
  fixtureExtensions: any[];
};

export function createExtensionRuntimeState(): ExtensionRuntimeState {
  return {
    extensionFileWatchers: [],
    extensionRuntimePreparation: undefined,
    testExtensionActivationFailurePhase: undefined,
    fixtureExtensions: [],
  };
}
