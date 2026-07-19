import type {
  ExtensionView,
  ExtensionWindowOptions,
} from '../resources/nevermind-extension-api';

function createExtensionWindowActions() {
  return {
    create: createWindowAction,
    show: showWindowAction,
    hide: hideWindowAction,
    toggle: toggleWindowAction,
    close: closeWindowAction,
  };
}

function createWindowAction(
  view: ExtensionView,
  options: ExtensionWindowOptions = {},
) {
  return {
    dismissAfterRun: 'auto',
    type: 'createWindow',
    title: options.title || view.title || 'Open Window',
    view,
    windowOptions: options,
    windowId: options.id || view.id,
  };
}

function showWindowAction(
  id: string,
  title = 'Show Window',
  options: Record<string, unknown> = {},
) {
  return windowVisibilityAction('showWindow', id, title, options);
}

function hideWindowAction(
  id: string,
  title = 'Hide Window',
  options: Record<string, unknown> = {},
) {
  return windowVisibilityAction('hideWindow', id, title, options);
}

function closeWindowAction(
  id: string,
  title = 'Close Window',
  options: Record<string, unknown> = {},
) {
  return windowVisibilityAction('closeWindow', id, title, options);
}

function windowVisibilityAction(
  type: 'showWindow' | 'hideWindow' | 'closeWindow',
  id: string,
  title: string,
  options: Record<string, unknown>,
) {
  return {
    dismissAfterRun: 'auto',
    ...options,
    type,
    title,
    windowId: id,
  };
}

function toggleWindowAction(
  idOrView: string | ExtensionView,
  titleOrOptions: string | ExtensionWindowOptions = 'Toggle Window',
  options: ExtensionWindowOptions = {},
) {
  if (typeof idOrView === 'string') {
    return toggleWindowById(idOrView, titleOrOptions, options);
  }
  return toggleWindowByView(idOrView, titleOrOptions, options);
}

function toggleWindowById(
  windowId: string,
  titleOrOptions: string | ExtensionWindowOptions,
  options: ExtensionWindowOptions,
) {
  const windowOptions =
    typeof titleOrOptions === 'string' ? options : titleOrOptions;
  return {
    dismissAfterRun: 'auto',
    ...options,
    type: 'toggleWindow',
    title:
      typeof titleOrOptions === 'string'
        ? titleOrOptions
        : titleOrOptions.title || 'Toggle Window',
    windowId,
    windowOptions,
  };
}

function toggleWindowByView(
  view: ExtensionView,
  titleOrOptions: string | ExtensionWindowOptions,
  options: ExtensionWindowOptions,
) {
  const windowOptions =
    typeof titleOrOptions === 'string' ? options : titleOrOptions;
  return {
    dismissAfterRun: 'auto',
    type: 'toggleWindow',
    title:
      typeof titleOrOptions === 'string'
        ? titleOrOptions
        : windowOptions.title || view.title || 'Toggle Window',
    view,
    windowOptions,
    windowId: windowOptions.id || view.id,
  };
}

export { createExtensionWindowActions };
