function extensionCommandAction<T>(
  command: { primaryAction?: T },
  fallbackAction: T,
) {
  return command.primaryAction ?? fallbackAction;
}

export { extensionCommandAction };
