import { promptActionView } from '../../extension-host/extension-action-builders';

export function handleRootView(action) {
  return { view: action.view, navigation: 'root' };
}

export function handlePushView(action) {
  return { view: action.view, navigation: 'push' };
}

export function handleReplaceView(action) {
  return { view: action.view, navigation: 'replace' };
}

export function handlePopView(action) {
  return { navigation: 'pop' };
}

export function handlePromptAction(action) {
  return { view: promptActionView(action), navigation: 'push' };
}
