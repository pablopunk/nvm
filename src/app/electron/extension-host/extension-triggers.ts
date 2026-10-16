// biome-ignore-all lint: Host-side extension triggers are authored by extensions and stay dynamic.
import { watch } from 'node:fs';
import path from 'node:path';
import { fileToExtensionFile } from '../files/extension-file';
import {
  displayUserPath,
  expandUserPath,
  extensionForPath,
  isImagePath,
  isVideoPath,
} from '../files/file-utils';
import {
  extensionsForFindOptions,
  normalizeFindRoots,
} from '../files/find-files';
import {
  ignoredByPattern,
  normalizedIgnorePatterns,
} from '../files/ignore-patterns';
import { info as logInfo, warn as logWarn } from '../platform/logger';
import type { JobDefinition, JobRegistry } from '../platform/jobs';
import type { ExtensionRuntimeState } from './extension-runtime-state';
import { FRONTMOST_APP_CHANGED_EVENT } from './host-jobs';

export function durationMs(value: any) {
  if (typeof value === 'number') return Math.max(1000, value);
  const match = String(value || '')
    .trim()
    .match(/^(\d+(?:\.\d+)?)\s*(ms|s|m|h|d)?$/i);
  if (!match) return 0;
  const amount = Number(match[1]);
  const unit = (match[2] || 'ms').toLowerCase();
  const multiplier =
    unit === 'd'
      ? 86_400_000
      : unit === 'h'
        ? 3_600_000
        : unit === 'm'
          ? 60_000
          : unit === 's'
            ? 1000
            : 1;
  return Math.max(1000, Math.round(amount * multiplier));
}

export function triggerPermission(_extension, _trigger: any) {
  // Host trigger registration is not a capability-enforcement boundary.
  return true;
}

export function normalizedFileTrigger(trigger: any) {
  const roots = normalizeFindRoots(trigger.roots)
    .map(expandUserPath)
    .filter((root) => root && path.isAbsolute(root));
  return {
    ...trigger,
    roots,
    includeHidden: Boolean(trigger.includeHidden),
    extensions: extensionsForFindOptions(trigger) || null,
    ignored: normalizedIgnorePatterns(trigger.ignore),
  };
}

export function fileWatchChangedPath(
  root: string,
  filename: string | Buffer | null,
) {
  if (!filename) return root;
  const value = String(filename);
  return path.isAbsolute(value) ? value : path.join(root, value);
}

export function fileWatchPathMatches(filePath: string, trigger: any) {
  const name = path.basename(filePath);
  if (!trigger.includeHidden && name.startsWith('.')) return false;
  if (ignoredByPattern(filePath, name, trigger.ignored || [])) return false;
  const ext = extensionForPath(filePath);
  if (trigger.extensions && !trigger.extensions.has(ext)) return false;
  if (trigger.kind === 'image' && !isImagePath(filePath)) return false;
  if (trigger.kind === 'video' && !isVideoPath(filePath)) return false;
  if (
    trigger.kind === 'media' &&
    !isImagePath(filePath) &&
    !isVideoPath(filePath)
  )
    return false;
  return true;
}

export function normalizedFileTriggerForLaunch(trigger: any) {
  return {
    type: 'files.changed',
    roots: normalizeFindRoots(trigger.roots).map(displayUserPath),
    debounceMs: trigger.debounceMs || 0,
    includeHidden: Boolean(trigger.includeHidden),
    extensions: trigger.extensions,
    kind: trigger.kind,
    ignore: trigger.ignore,
  };
}

export function extensionTriggerForLaunch(trigger: any) {
  if (!trigger) return;
  if (trigger.type === 'files.changed')
    return normalizedFileTriggerForLaunch(trigger);
  return structuredClone(trigger);
}

export async function extensionLaunchContextFromJob(context: any) {
  const payload =
    context?.payload && typeof context.payload === 'object'
      ? context.payload
      : {};
  const changedPaths = Array.isArray(payload.changedPaths)
    ? payload.changedPaths
        .map((value) => expandUserPath(String(value)))
        .filter(Boolean)
        .slice(-100)
    : [];
  const files = changedPaths.length
    ? await Promise.all(
        changedPaths.map((filePath) => fileToExtensionFile(filePath)),
      )
    : [];
  return structuredClone({
    trigger: payload.trigger,
    files,
    changedPaths,
    reason: context?.reason || 'manual',
    event: context?.event,
    startedAt: context?.startedAt || Date.now(),
  });
}

export type ExtensionTriggersDeps = {
  jobRegistry: JobRegistry;
  runtimeState: ExtensionRuntimeState;
  EXTENSION_ROOT_ITEMS_TIMEOUT_MS: number;
  executeViewAction: (action: any, launchContext?: any) => Promise<any>;
};

export function createExtensionTriggers(deps: ExtensionTriggersDeps) {
  const {
    jobRegistry,
    runtimeState,
    EXTENSION_ROOT_ITEMS_TIMEOUT_MS,
    executeViewAction,
  } = deps;

  function attachExtensionFileTrigger(
    trigger: any,
    event: string,
    extensionId: string,
  ) {
    const normalized = normalizedFileTrigger(trigger);
    for (const root of normalized.roots) {
      try {
        const watcher = watch(
          root,
          {
            recursive:
              process.platform === 'darwin' || process.platform === 'win32',
          },
          (_eventType, filename) => {
            const changedPath = fileWatchChangedPath(root, filename);
            if (!fileWatchPathMatches(changedPath, normalized)) return;
            jobRegistry.emit(event, {
              trigger: normalizedFileTriggerForLaunch(trigger),
              root: displayUserPath(root),
              changedPaths: [changedPath],
            });
          },
        );
        watcher.on('error', () => {});
        runtimeState.extensionFileWatchers.push({
          close: () => watcher.close(),
          extensionId,
          trigger,
          event,
        });
      } catch {}
    }
  }

  function watchExtensionFileTrigger(
    trigger: any,
    event: string,
    extensionId: string,
  ) {
    if (runtimeState.extensionRuntimePreparation) {
      runtimeState.extensionRuntimePreparation.fileWatchers.push({
        trigger,
        event,
        extensionId,
      });
      return;
    }
    attachExtensionFileTrigger(trigger, event, extensionId);
  }

  function jobTriggersFromExtensionTriggers(
    extension,
    triggers: any[] = [],
    jobId = '',
  ) {
    return triggers
      .map((trigger, index) => {
        if (!triggerPermission(extension, trigger)) {
          logWarn(
            'extension.jobTrigger.permissionDenied',
            { trigger: trigger?.type },
            { source: 'host', scope: 'extension', extensionId: extension.id },
          );
          return null;
        }
        if (trigger?.type === 'startup')
          return {
            type: 'startup' as const,
            delayMs: trigger.delayMs || 0,
            payload: { trigger: extensionTriggerForLaunch(trigger) },
          };
        if (trigger?.type === 'login')
          return {
            type: 'event' as const,
            event: 'login',
            debounceMs: trigger.debounceMs || 0,
            payload: { trigger: extensionTriggerForLaunch(trigger) },
          };
        if (trigger?.type === 'wake')
          return {
            type: 'event' as const,
            event: 'wake',
            debounceMs: trigger.debounceMs || 0,
            payload: { trigger: extensionTriggerForLaunch(trigger) },
          };
        if (trigger?.type === 'interval') {
          const everyMs = durationMs(trigger.every);
          return everyMs
            ? {
                type: 'interval' as const,
                everyMs,
                delayMs: trigger.delayMs,
                payload: { trigger: extensionTriggerForLaunch(trigger) },
              }
            : null;
        }
        if (trigger?.type === 'clipboard.changed')
          return {
            type: 'event' as const,
            event: 'clipboard.changed',
            debounceMs: trigger.debounceMs || 0,
            payload: { trigger: extensionTriggerForLaunch(trigger) },
          };
        if (trigger?.type === 'app.frontmost.changed')
          return {
            type: 'event' as const,
            event: FRONTMOST_APP_CHANGED_EVENT,
            debounceMs: trigger.debounceMs || 0,
            payload: { trigger: extensionTriggerForLaunch(trigger) },
          };
        if (trigger?.type === 'files.changed') {
          const event = `files.changed:${jobId}:${index}`;
          watchExtensionFileTrigger(trigger, event, extension.id);
          return {
            type: 'event' as const,
            event,
            debounceMs: trigger.debounceMs || 0,
            payload: { trigger: extensionTriggerForLaunch(trigger) },
          };
        }
        return null;
      })
      .filter(Boolean);
  }

  function registerExtensionBackgroundJob(entry, item) {
    const mode = item.mode || (item.background ? 'background' : 'view');
    const id = `extension.${entry.extension.id}.${item.id}`;
    const triggers = jobTriggersFromExtensionTriggers(
      entry.extension,
      item.triggers || [],
      id,
    );
    if (mode === 'view' && triggers.length === 0) return;
    const definition: JobDefinition = {
      id,
      title: item.title,
      owner: 'extension',
      scope: entry.extension.id,
      triggers,
      timeoutMs: Number(item.timeoutMs || EXTENSION_ROOT_ITEMS_TIMEOUT_MS),
      run: async (context) => {
        const action = item.primaryAction || item.action;
        if (!action) return;
        const launchContext = await extensionLaunchContextFromJob(context);
        const result = await executeViewAction(action, launchContext);
        if (result?.view)
          logInfo(
            'extension.background.viewIgnored',
            { jobId: id, title: item.title },
            {
              source: 'host',
              scope: 'extension',
              extensionId: entry.extension.id,
              commandId: item.id,
            },
          );
      },
    };
    if (runtimeState.extensionRuntimePreparation)
      runtimeState.extensionRuntimePreparation.jobs.push(definition);
    else jobRegistry.register(definition);
  }

  return { attachExtensionFileTrigger, registerExtensionBackgroundJob };
}
