import type { AgentSession } from '@earendil-works/pi-coding-agent';

export type PiToolPolicySnapshot = {
  registeredTools: Array<{
    name: string;
    exposure: ReturnType<AgentSession['getAllTools']>[number]['exposure'];
  }>;
  activeToolNames: string[];
  callableToolNames: string[];
};

export function ensurePiToolAllowlist(
  session: AgentSession,
  allowedToolNames: readonly string[],
  mode: string,
): PiToolPolicySnapshot {
  const allowed = new Set(allowedToolNames);
  const registeredTools = session
    .getAllTools()
    .map(({ name, exposure }) => ({ name, exposure }))
    .sort(compareRegisteredTools);
  const registeredNames = new Set(registeredTools.map((tool) => tool.name));
  const activeToolNames = sortedNames(session.getActiveToolNames());
  const callableToolNames = sortedNames(session.getCallableToolNames());
  const missingActive = allowedToolNames.filter(
    (name) => !activeToolNames.includes(name) || !registeredNames.has(name),
  );
  const unexpectedActive = activeToolNames.filter((name) => !allowed.has(name));
  const unexpectedCallable = callableToolNames.filter(
    (name) => !allowed.has(name),
  );
  const nonDirectAllowed = registeredTools
    .filter((tool) => allowed.has(tool.name) && tool.exposure !== 'direct')
    .map((tool) => tool.name);
  const forbiddenRegistered = registeredTools
    .filter((tool) => isForbiddenDiscoveryTool(tool.name))
    .map((tool) => tool.name);
  const violations = [
    ...missingActive.map((name) => `missing:${name}`),
    ...unexpectedActive.map((name) => `active:${name}`),
    ...unexpectedCallable.map((name) => `callable:${name}`),
    ...nonDirectAllowed.map((name) => `non-direct:${name}`),
    ...forbiddenRegistered.map((name) => `registered:${name}`),
  ];
  if (violations.length > 0) {
    session.dispose();
    throw new Error(
      `Pi tool policy failed for ${mode}: ${[...new Set(violations)].join(', ')}`,
    );
  }
  return { registeredTools, activeToolNames, callableToolNames };
}

function sortedNames(names: string[]) {
  return [...names].sort();
}

function compareRegisteredTools(
  left: PiToolPolicySnapshot['registeredTools'][number],
  right: PiToolPolicySnapshot['registeredTools'][number],
) {
  return left.name.localeCompare(right.name);
}

function isForbiddenDiscoveryTool(name: string) {
  const normalized = name.toLowerCase();
  return (
    normalized === 'tool_search' ||
    normalized.includes('codemode') ||
    normalized.startsWith('mcp__')
  );
}
