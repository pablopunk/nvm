import type { PortableConfig } from '../state/portable-config-schema';

export type DesiredBinding = {
  actionId: string;
  accelerator: string;
  title: string;
  source: 'user' | 'declared';
};

export type DeclaredShortcut = {
  actionId: string;
  accelerator: string;
  action: { title: string };
};

export type BindingPlan = {
  bindings: Array<{ actionId: string; accelerator: string }>;
  inactive: Array<{ actionId: string; accelerator: string; message: string }>;
};

export function desiredShortcutBindings(
  view: PortableConfig,
  resolveAction: (actionId: string) => { title?: string } | null | undefined,
  declared: DeclaredShortcut[],
): DesiredBinding[] {
  const bindings: DesiredBinding[] = [];
  for (const [actionId, accelerator] of Object.entries(view.shortcuts)) {
    if (typeof accelerator !== 'string') continue;
    const action = resolveAction(actionId);
    if (!action) continue;
    bindings.push({
      actionId,
      accelerator,
      title: action.title ?? actionId,
      source: 'user',
    });
  }
  for (const item of declared) {
    if (item.actionId in view.shortcuts) continue;
    bindings.push({
      actionId: item.actionId,
      accelerator: item.accelerator,
      title: item.action.title,
      source: 'declared',
    });
  }
  return bindings;
}

function claimOrder(left: DesiredBinding, right: DesiredBinding) {
  const sourceRank = (binding: DesiredBinding) =>
    binding.source === 'user' ? 0 : 1;
  return (
    sourceRank(left) - sourceRank(right) ||
    left.actionId.localeCompare(right.actionId)
  );
}

export function planShortcutBindings(
  desired: DesiredBinding[],
  isReserved: (accelerator: string) => boolean,
): BindingPlan {
  const claimed = new Map<string, string>();
  const plan: BindingPlan = { bindings: [], inactive: [] };
  for (const binding of [...desired].sort(claimOrder)) {
    const { actionId, accelerator } = binding;
    if (isReserved(accelerator)) {
      plan.inactive.push({
        actionId,
        accelerator,
        message: `${accelerator} is reserved for opening Nevermind`,
      });
      continue;
    }
    const owner = claimed.get(accelerator);
    if (owner !== undefined) {
      plan.inactive.push({
        actionId,
        accelerator,
        message: `${accelerator} is already used by ${owner}`,
      });
      continue;
    }
    claimed.set(accelerator, binding.title);
    plan.bindings.push({ actionId, accelerator });
  }
  return plan;
}
