export function normalizedExtensionMetadata(metadata) {
  const value =
    typeof metadata === 'string'
      ? { title: metadata, commandTitle: metadata }
      : metadata || {};
  return {
    title: value.title == null ? undefined : String(value.title).trim(),
    subtitle:
      value.subtitle == null ? undefined : String(value.subtitle).trim(),
    commandTitle:
      value.commandTitle == null
        ? undefined
        : String(value.commandTitle).trim(),
    commandSubtitle:
      value.commandSubtitle == null
        ? undefined
        : String(value.commandSubtitle).trim(),
  };
}

export function applyExtensionMetadata(extension, metadata) {
  const normalized = normalizedExtensionMetadata(metadata);
  if (normalized.title) extension.title = normalized.title;
  if (normalized.subtitle) extension.subtitle = normalized.subtitle;
  if (!Array.isArray(extension.commands)) return;
  for (const command of extension.commands) {
    if (normalized.commandTitle) command.title = normalized.commandTitle;
    if (normalized.commandSubtitle)
      command.subtitle = normalized.commandSubtitle;
  }
}
