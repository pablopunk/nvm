// biome-ignore-all lint: Dev fixture extensions are dynamic extension payloads by design.
import fs from 'node:fs/promises';
import path from 'node:path';
import { error as logError } from '../platform/logger';
import {
  compareExtensionSourceEntries,
  isExtensionSourceFile,
} from './extension-source-files';
import { loadExtensionModule } from './extension-module-loader';
import type {
  ExtensionRuntimeRegistries,
  ExtensionRuntimeState,
} from './extension-runtime-state';

export type ExtensionDevFixturesDeps = Pick<
  ExtensionRuntimeRegistries,
  'extensionActionRegistry'
> & {
  runtimeState: ExtensionRuntimeState;
  getAppPath: () => string;
  extensionActionFromContribution: (entry: any) => any;
  registerExtension: (extension: any) => Promise<void>;
};

export function createExtensionDevFixtures(deps: ExtensionDevFixturesDeps) {
  const {
    runtimeState,
    extensionActionRegistry,
    getAppPath,
    extensionActionFromContribution,
    registerExtension,
  } = deps;

  function fixturePersistentActionItems(fixture) {
    const registeredEntries = Array.from(
      extensionActionRegistry.values(),
    ).filter((entry) => entry.extension.id === fixture.id);
    return registeredEntries.map((entry) => {
      const item = entry.item;
      const persistentAction = extensionActionFromContribution(entry);
      return {
        ...item,
        id: `fixture-action:${fixture.id}:${item.id}`,
        persistentAction,
        primaryAction: persistentAction?.rootAction || item.primaryAction,
        subtitle:
          item.subtitle || `Persistent action · ${fixture.title || fixture.id}`,
        accessories: [
          ...(item.accessories || []),
          { text: entry.source === 'command' ? 'command' : 'action' },
        ],
      };
    });
  }

  function fixturesIndexView(ctx) {
    return ctx.ui.list({
      id: 'extension-api-fixtures',
      title: 'Extension API Fixtures',
      subtitle: 'Dev-only runnable fixtures for host-rendered extension UI',
      searchBarPlaceholder: 'Search fixture commands and persistent actions',
      emptyView: {
        title: 'No fixtures found',
        subtitle: 'Add fixture extensions under src/app/fixtures.',
      },
      sections: runtimeState.fixtureExtensions.map((fixture) => ({
        title: fixture.title || fixture.id,
        subtitle: fixture.subtitle || 'Extension API fixture',
        items: fixturePersistentActionItems(fixture),
      })),
    });
  }

  function fixturesRootItem(ctx) {
    const persistentItems = runtimeState.fixtureExtensions.flatMap((fixture) =>
      fixturePersistentActionItems(fixture),
    );
    const runnableCount = persistentItems.length;
    return {
      id: 'fixtures',
      title: 'Fixtures',
      subtitle: `${runtimeState.fixtureExtensions.length} dev-only extension API ${runtimeState.fixtureExtensions.length === 1 ? 'fixture' : 'fixtures'} · ${runnableCount} ${runnableCount === 1 ? 'item' : 'items'}`,
      icon: 'wrench',
      aliases: [
        'fixture',
        'fixtures',
        'dev fixtures',
        'extension fixtures',
        ...runtimeState.fixtureExtensions.map(
          (fixture) => fixture.title || fixture.id,
        ),
        ...persistentItems.map((item) => item.title),
      ].filter(Boolean),
      score: 100,
      primaryAction: ctx.actions.push('Open Fixtures', fixturesIndexView(ctx)),
    };
  }

  function createFixturesExtension() {
    return {
      id: 'dev.fixtures',
      title: 'Fixtures',
      subtitle: 'Dev-only extension API fixtures',
      rootItems(ctx) {
        return [fixturesRootItem(ctx)];
      },
      searchItems(ctx) {
        return [fixturesRootItem(ctx)];
      },
    };
  }

  async function loadDevExtensions() {
    const fixturesDir = path.join(getAppPath(), 'src', 'app', 'fixtures');
    const entries = (
      await fs.readdir(fixturesDir, { withFileTypes: true }).catch(() => [])
    ).sort(compareExtensionSourceEntries);
    for (const entry of entries) {
      if (!isExtensionSourceFile(entry.name)) continue;
      const fullPath = path.join(fixturesDir, entry.name);
      try {
        const extension = await loadExtensionModule(fullPath);
        extension.__filePath = fullPath;
        extension.__dev = true;
        extension.__fixture = true;
        await registerExtension(extension);
        runtimeState.fixtureExtensions.push(extension);
      } catch (error) {
        logError('extension.dev.load.failed', error, {
          source: 'host',
          scope: 'extension',
          extensionId: path.basename(fullPath),
        });
      }
    }
    if (runtimeState.fixtureExtensions.length)
      await registerExtension(createFixturesExtension());
  }

  return { loadDevExtensions };
}
