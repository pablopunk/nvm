// biome-ignore-all lint: Host-side extension payload boundary keeps dynamic action shapes by design.
export function commandFromItem(item) {
  return { ...item, run: (ctx) => ctx.navigation.run(item.primaryAction) };
}

export function wrapWithConfirmation(action: any, input: any) {
  if (!action) return action;
  const destructive = input?.destructive ?? action.style === 'destructive';
  return {
    ...action,
    title: input?.title || action.title,
    requiresConfirmation: true,
    ...(destructive ? { style: 'destructive' } : {}),
    ...(input?.message === undefined
      ? {}
      : { confirmMessage: String(input.message) }),
    ...(input?.confirmLabel === undefined
      ? {}
      : { confirmLabel: String(input.confirmLabel) }),
    ...(input?.cancelLabel === undefined
      ? {}
      : { cancelLabel: String(input.cancelLabel) }),
  };
}

export function buildPromptAction(input: any = {}, options: any = {}) {
  const targetAction = input?.action || input?.onSubmit;
  if (!targetAction) throw new Error('ctx.input.prompt requires action');
  const title = String(input?.title || targetAction.title || 'Prompt');
  return {
    ...options,
    type: 'promptAction',
    title,
    subtitle:
      input?.message === undefined ? options.subtitle : String(input.message),
    promptMessage:
      input?.message === undefined ? undefined : String(input.message),
    fields: Array.isArray(input?.fields) ? input.fields : [],
    targetAction,
    submitTitle:
      input?.submitTitle === undefined ? undefined : String(input.submitTitle),
  };
}

export function promptActionView(action: any = {}) {
  const targetAction = action.targetAction || action.action;
  if (!targetAction)
    throw new Error('Prompt action is missing its target action');
  const fields = Array.isArray(action.fields) ? action.fields : [];
  const promptMessage = action.promptMessage || action.subtitle;
  return {
    type: 'prompt',
    title: action.title || 'Prompt',
    subtitle: promptMessage ? String(promptMessage) : undefined,
    fields,
    submitAction: {
      ...targetAction,
      title: action.submitTitle || targetAction.title || 'Submit',
    },
  };
}

export function buildConfirmAction(input: any) {
  const inner = input?.onConfirm || input?.action;
  if (!inner) throw new Error('ctx.ui.confirm requires onConfirm action');
  return wrapWithConfirmation(inner, input);
}

export function buildPreviewItemAction(input: any) {
  const kind =
    input?.kind ||
    (input?.text
      ? 'text'
      : input?.imageDataUrl || input?.imagePath
        ? 'image'
        : input?.videoUrl
          ? 'video'
          : input?.filePath
            ? 'file'
            : 'clipboard');
  return {
    type: 'previewClipboardItem',
    title: input?.title || 'Preview',
    description:
      input?.description ||
      (kind === 'file' || kind === 'image' || kind === 'video'
        ? 'Preview this file'
        : 'Preview clipboard item'),
    shortcut: input?.shortcut || 'Command+Y',
    clipboardType: input?.clipboardType || kind,
    text: input?.text,
    imageDataUrl: input?.imageDataUrl,
    imagePath: input?.imagePath,
    videoUrl: input?.videoUrl,
    filePath: input?.filePath,
    thumbnailUrl: input?.thumbnailUrl,
  };
}

export function progressView(input: any = {}) {
  const hasSteps = Array.isArray(input.steps) && input.steps.length;
  const steps = hasSteps
    ? input.steps
    : [
        {
          title: input.label || input.title || 'Loading...',
          status: input.status || 'active',
        },
      ];
  return {
    ...input,
    type: 'progress',
    title: input.title || input.label || 'Loading...',
    steps,
    ...(input.id === undefined ? {} : { id: String(input.id) }),
    ...(typeof input.value === 'number' ? { value: input.value } : {}),
    ...(typeof input.total === 'number' ? { total: input.total } : {}),
  };
}
