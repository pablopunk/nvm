export function createExtensionWindowActions() {
  return {
    create: (view: any, options: any = {}) => ({
      dismissAfterRun: 'auto',
      type: 'createWindow',
      title: options.title || view?.title || 'Open Window',
      view,
      windowOptions: options,
      windowId: options.id || view?.id,
    }),
    show: (id: string, title = 'Show Window', options: any = {}) => ({
      dismissAfterRun: 'auto',
      ...options,
      type: 'showWindow',
      title,
      windowId: id,
    }),
    hide: (id: string, title = 'Hide Window', options: any = {}) => ({
      dismissAfterRun: 'auto',
      ...options,
      type: 'hideWindow',
      title,
      windowId: id,
    }),
    toggle: (
      idOrView: any,
      titleOrOptions: any = 'Toggle Window',
      options: any = {},
    ) => {
      if (typeof idOrView === 'string')
        return {
          dismissAfterRun: 'auto',
          ...options,
          type: 'toggleWindow',
          title:
            typeof titleOrOptions === 'string'
              ? titleOrOptions
              : titleOrOptions.title || 'Toggle Window',
          windowId: idOrView,
          windowOptions:
            typeof titleOrOptions === 'string' ? options : titleOrOptions,
        };
      const windowOptions =
        typeof titleOrOptions === 'string' ? options : titleOrOptions || {};
      return {
        dismissAfterRun: 'auto',
        type: 'toggleWindow',
        title:
          typeof titleOrOptions === 'string'
            ? titleOrOptions
            : windowOptions.title || idOrView?.title || 'Toggle Window',
        view: idOrView,
        windowOptions,
        windowId: windowOptions.id || idOrView?.id,
      };
    },
    close: (id: string, title = 'Close Window', options: any = {}) => ({
      dismissAfterRun: 'auto',
      ...options,
      type: 'closeWindow',
      title,
      windowId: id,
    }),
  };
}
