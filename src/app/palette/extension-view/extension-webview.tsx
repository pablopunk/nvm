import React, { useEffect, useMemo, useState } from 'react';
import {
  EXTENSION_WEBVIEW_SANDBOX,
  extensionWebviewAllow,
} from './extension-webview-permissions';
import { currentWebviewThemeValues, withWebviewTheme } from './webview-theme';

function useCameraAccessForWebview(webviewPermissions?: readonly string[]) {
  const needsCamera = Boolean(webviewPermissions?.includes('camera'));
  const [ready, setReady] = useState(!needsCamera);
  useEffect(() => {
    if (!needsCamera) return;
    let cancelled = false;
    window.nvm
      .requestCameraAccess()
      .catch(() => undefined)
      .then(() => {
        if (!cancelled) setReady(true);
      });
    return () => {
      cancelled = true;
    };
  }, [needsCamera]);
  return ready;
}

function useThemedWebviewSource(html: string) {
  return useMemo(
    () => withWebviewTheme(html, currentWebviewThemeValues()),
    [html],
  );
}

export function ExtensionWebviewFrame({
  title,
  html,
  webviewPermissions,
}: {
  title?: string;
  html: string;
  webviewPermissions?: readonly string[];
}) {
  const cameraAccessReady = useCameraAccessForWebview(webviewPermissions);
  const source = useThemedWebviewSource(html);
  if (!cameraAccessReady) return null;
  return (
    <iframe
      className="extensionWebview"
      title={title}
      srcDoc={source}
      sandbox={EXTENSION_WEBVIEW_SANDBOX}
      allow={extensionWebviewAllow(webviewPermissions)}
    />
  );
}
