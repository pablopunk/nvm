export type ExtensionUiApiDeps = {
  buildPreviewItemAction: (item: unknown) => unknown
  progressView: (input?: unknown) => unknown
  buildConfirmAction: (input?: unknown) => unknown
}

function isPreviewableItem(value: any) {
  return value?.kind && ['clipboard', 'image', 'video', 'file', 'text'].includes(value.kind)
}

function isFilePreviewInput(value: any) {
  return value?.path || value?.fileUrl || value?.videoUrl || value?.thumbnailUrl
}

export function createExtensionUiApi({ buildPreviewItemAction, progressView, buildConfirmAction }: ExtensionUiApiDeps) {
  return {
    list: (view: any) => ({ ...view, type: 'list' }),
    grid: (view: any) => ({ ...view, type: 'grid' }),
    preview: (fileOrView: any, view: any = {}) => {
      if (isPreviewableItem(fileOrView)) return buildPreviewItemAction(fileOrView)
      if (!isFilePreviewInput(fileOrView)) return { ...fileOrView, type: 'preview' }
      const file = fileOrView
      return {
        ...view,
        type: 'preview',
        presentation: view.presentation || 'preview',
        title: view.title || file.name || 'Preview',
        subtitle: view.subtitle || file.displayPath,
        content: view.content || file.displayPath || '',
        image: file.thumbnailUrl || file.url,
        video: file.videoUrl || undefined,
      }
    },
    chat: (view: any) => ({ ...view, type: 'chat' }),
    form: (view: any) => ({ ...view, type: 'form' }),
    editor: (view: any) => ({ ...view, type: 'editor' }),
    progress: (input: any = {}) => progressView(input),
    confirm: (input: any = {}) => buildConfirmAction(input),
    toast: (input: any = {}) => ({ toast: { message: String(input?.message || ''), tone: input?.tone || 'default' } }),
    webview: (view: any) => ({ ...view, type: 'webview' }),
    camera: (view = {}) => ({ title: 'Camera', size: 'large', muted: true, ...view, type: 'camera' }),
    item: (item: any) => item,
    actions: (actions: any) => actions,
    empty: (title = 'Nothing here', subtitle = '') => ({ type: 'preview', title, content: `# ${title}${subtitle ? `\n\n${subtitle}` : ''}` }),
    loading: (title = 'Loading…') => progressView({ title, label: title }),
    error: (title = 'Something went wrong', message = '') => ({ type: 'preview', title, content: `# ${title}${message ? `\n\n${message}` : ''}` }),
  }
}
