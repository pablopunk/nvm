type InternalCommandRequirement = {
  extensionId: string;
  commandId: string;
  capability?: string;
};

export function missingRequiredInternalCommands(
  requirements: readonly InternalCommandRequirement[],
  registeredCommands: ReadonlyMap<string, unknown>,
  hasCapability: (capability: string) => boolean,
) {
  return requirements.filter(
    ({ extensionId, commandId, capability }) =>
      (!capability || hasCapability(capability)) &&
      !registeredCommands.has(`${extensionId}:${commandId}`),
  );
}
