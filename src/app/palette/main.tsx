import React from 'react';
import ReactDom from 'react-dom/client';
import { App, ExtensionWindowApp } from './App';
import './posthog';
import './styles.css';
import { captureRendererFailure } from './diagnostics';

const extensionWindowId = new URLSearchParams(window.location.search).get(
  'extensionWindowId',
);

ReactDom.createRoot(document.getElementById('root')!, {
  onUncaughtError: captureRendererFailure,
}).render(
  <React.StrictMode>
    {extensionWindowId ? (
      <ExtensionWindowApp windowId={extensionWindowId} />
    ) : (
      <App />
    )}
  </React.StrictMode>,
);
