// biome-ignore-all lint/performance/noAwaitInLoops: each Trash move must re-check the app and the item immediately before it happens.
import type { AppGuards, CheckedApp } from './app-uninstall-guards';
import type {
  AppUninstallDependencies,
  AppUninstallNote,
  AppUninstallSnapshot,
  Refusal,
  SnapshotCandidate,
  TrashResult,
  UntouchedItem,
} from './app-uninstall-types';

const DEFAULT_QUIT_TIMEOUT_MS = 10_000;
const DEFAULT_QUIT_POLL_MS = 250;

const REFUSALS = {
  running: {
    code: 'running',
    message: 'Quit this application before moving its data to Trash',
  },
  quitTimeout: {
    code: 'quit-timeout',
    message:
      'The app or a background helper is still running. Quit it in Activity Monitor and try again. Nothing was moved to Trash.',
  },
  quitFailed: {
    code: 'quit-failed',
    message:
      'Could not ask the application to quit. Nothing was moved to Trash.',
  },
  trashFailed: {
    code: 'trash-failed',
    message: 'Could not move to Trash. Check its permissions.',
  },
  zeroSelection: {
    code: 'zero-selection',
    message: 'Select at least one item to move to Trash.',
  },
} as const;

export function selectedCandidates(
  snapshot: AppUninstallSnapshot,
  values: Record<string, unknown> = {},
) {
  return snapshot.candidates.filter(
    (candidate) => values[candidate.id] === true,
  );
}

function trashStatus(
  moved: string[],
  untouched: UntouchedItem[],
): TrashResult['status'] {
  if (untouched.length === 0) {
    return 'complete';
  }
  return moved.length > 0 ? 'partial' : 'failed';
}

function untouchedFor(items: SnapshotCandidate[], refusal: Refusal) {
  return items.map((item) => ({ path: item.path, ...refusal }));
}

function refusedResult(
  items: SnapshotCandidate[],
  refusal: Refusal,
): TrashResult {
  return {
    status: 'failed',
    moved: [],
    untouched: untouchedFor(items, refusal),
    notes: [],
  };
}

async function requestQuitSafely(
  deps: AppUninstallDependencies,
  appPath: string,
) {
  try {
    await deps.requestQuit(appPath);
    return true;
  } catch {
    return false;
  }
}

async function moveToTrash(deps: AppUninstallDependencies, itemPath: string) {
  try {
    await deps.trashItem(itemPath);
    return true;
  } catch {
    return false;
  }
}

function defaultWait(milliseconds: number) {
  return new Promise<void>((resolve) => setTimeout(resolve, milliseconds));
}

export function createTrashRunner({
  deps,
  guards,
}: {
  deps: AppUninstallDependencies;
  guards: AppGuards;
}) {
  const wait = deps.wait ?? defaultWait;
  const quitTimeoutMs = deps.quitTimeoutMs ?? DEFAULT_QUIT_TIMEOUT_MS;
  const quitPollMs = deps.quitPollMs ?? DEFAULT_QUIT_POLL_MS;

  async function quitGracefully(app: CheckedApp): Promise<Refusal | null> {
    const requested = await requestQuitSafely(deps, app.canonical);
    if (!requested) {
      return (await guards.isRunning(app.canonical))
        ? REFUSALS.quitFailed
        : null;
    }
    for (let waitedMs = 0; ; waitedMs += quitPollMs) {
      if (!(await guards.isRunning(app.canonical))) {
        return null;
      }
      if (waitedMs >= quitTimeoutMs) {
        break;
      }
      await wait(quitPollMs);
    }
    return REFUSALS.quitTimeout;
  }

  async function currentAppRefusal(
    snapshot: AppUninstallSnapshot,
  ): Promise<Refusal | null> {
    const app = await guards.checkApp(snapshot.appPath, snapshot);
    if (!('canonical' in app)) {
      return app;
    }
    return (await guards.isRunning(app.canonical)) ? REFUSALS.running : null;
  }

  async function preflightRefusals(items: SnapshotCandidate[]) {
    const checks = await Promise.all(
      items.map(async (item) => ({
        item,
        checked: await guards.revalidateCandidate(item),
      })),
    );
    const refusals = new Map<string, Refusal>();
    for (const { item, checked } of checks) {
      if (!('canonical' in checked)) {
        refusals.set(item.id, checked);
      }
    }
    return refusals;
  }

  async function trashAssociatedItems(
    snapshot: AppUninstallSnapshot,
    items: SnapshotCandidate[],
  ) {
    const refusals = await preflightRefusals(items);
    const moved: string[] = [];
    const untouched: UntouchedItem[] = [];
    for (const [index, item] of items.entries()) {
      const preflightRefusal = refusals.get(item.id);
      if (preflightRefusal) {
        untouched.push({ path: item.path, ...preflightRefusal });
        continue;
      }
      const stop = await currentAppRefusal(snapshot);
      if (stop) {
        return {
          moved,
          untouched: [...untouched, ...untouchedFor(items.slice(index), stop)],
          stop,
        };
      }
      const checked = await guards.revalidateCandidate(item);
      if (!('canonical' in checked)) {
        untouched.push({ path: item.path, ...checked });
        continue;
      }
      if (await moveToTrash(deps, item.path)) {
        moved.push(item.path);
      } else {
        untouched.push({ path: item.path, ...REFUSALS.trashFailed });
      }
    }
    return { moved, untouched, stop: null };
  }

  async function appTrashRefusal(
    snapshot: AppUninstallSnapshot,
    appItem: SnapshotCandidate,
  ): Promise<Refusal | null> {
    const stop = await currentAppRefusal(snapshot);
    if (stop) {
      return stop;
    }
    const checked = await guards.revalidateCandidate(appItem);
    return 'canonical' in checked ? null : checked;
  }

  async function trash(
    snapshot: AppUninstallSnapshot,
    values: Record<string, unknown> = {},
  ): Promise<TrashResult> {
    const selection = selectedCandidates(snapshot, values);
    if (selection.length === 0) {
      return {
        status: 'failed',
        moved: [],
        untouched: [],
        notes: [
          {
            code: REFUSALS.zeroSelection.code,
            message: REFUSALS.zeroSelection.message,
          },
        ],
      };
    }
    const appItem = selection.find((item) => item.kind === 'app');
    const associatedItems = selection.filter((item) => item.kind !== 'app');
    const app = await guards.checkApp(snapshot.appPath, snapshot);
    if (!('canonical' in app)) {
      return refusedResult(selection, app);
    }
    const notes: AppUninstallNote[] = [];
    if (await guards.isRunning(app.canonical)) {
      if (!appItem) {
        return refusedResult(selection, REFUSALS.running);
      }
      const quitRefusal = await quitGracefully(app);
      if (quitRefusal) {
        return refusedResult(selection, quitRefusal);
      }
      notes.push({
        code: 'app-quit',
        message: 'The application was quit before anything was moved to Trash.',
      });
    }
    const associated = await trashAssociatedItems(snapshot, associatedItems);
    const moved = [...associated.moved];
    const untouched = [...associated.untouched];
    if (appItem) {
      const appRefusal =
        associated.stop ?? (await appTrashRefusal(snapshot, appItem));
      if (appRefusal) {
        untouched.push({ path: appItem.path, ...appRefusal });
      } else if (await moveToTrash(deps, appItem.path)) {
        moved.push(appItem.path);
      } else {
        untouched.push({ path: appItem.path, ...REFUSALS.trashFailed });
      }
    }
    return {
      status: trashStatus(moved, untouched),
      moved,
      untouched,
      notes,
    };
  }

  return { trash };
}
