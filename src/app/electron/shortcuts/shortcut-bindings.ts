import {
  type DesiredBinding,
  planShortcutBindings,
} from './shortcut-binding-plan';

export type ShortcutBindingsDeps = {
  globalShortcut: {
    register: (accelerator: string, listener: () => void) => boolean;
    unregister: (accelerator: string) => void;
  };
  runAction: (actionId: string) => unknown;
};

export function createShortcutBindings(deps: ShortcutBindingsDeps) {
  const owned = new Map<string, string>();

  function releaseUnwanted(wantedAcceleratorByAction: Map<string, string>) {
    for (const [accelerator, actionId] of [...owned]) {
      if (wantedAcceleratorByAction.get(actionId) === accelerator) continue;
      deps.globalShortcut.unregister(accelerator);
      owned.delete(accelerator);
    }
  }

  function bindWanted(
    bindings: Array<{ actionId: string; accelerator: string }>,
  ) {
    const failures = new Map<string, string>();
    for (const { actionId, accelerator } of bindings) {
      if (owned.get(accelerator) === actionId) continue;
      const bound = deps.globalShortcut.register(accelerator, () =>
        deps.runAction(actionId),
      );
      if (bound) owned.set(accelerator, actionId);
      else failures.set(actionId, `${accelerator} is in use by another app`);
    }
    return failures;
  }

  function apply(
    desired: DesiredBinding[],
    isReserved: (accelerator: string) => boolean,
  ) {
    const plan = planShortcutBindings(desired, isReserved);
    releaseUnwanted(
      new Map(plan.bindings.map((item) => [item.actionId, item.accelerator])),
    );
    const failures = new Map(
      plan.inactive.map((item) => [item.actionId, item.message]),
    );
    for (const [actionId, message] of bindWanted(plan.bindings)) {
      failures.set(actionId, message);
    }
    return failures;
  }

  function releaseAll() {
    for (const accelerator of owned.keys())
      deps.globalShortcut.unregister(accelerator);
    owned.clear();
  }

  return { apply, releaseAll, ownedAccelerators: () => new Map(owned) };
}
