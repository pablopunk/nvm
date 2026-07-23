import React, { useEffect, useState } from 'react';
import ReactDOM from 'react-dom/client';
import { App, ExtensionWindowApp } from './App';
import { DesignTokenEditor } from './design-token-editor';
import type { DesignTokenState } from './preload-api';
import './styles.css';
import './design-token-editor.css';

const extensionWindowId = new URLSearchParams(window.location.search).get(
  'extensionWindowId',
);

function Root() {
  const [tokenState, setTokenState] = useState<DesignTokenState | null>(null);
  useEffect(() => {
    const open = (event: KeyboardEvent) => {
      if (event.altKey && event.shiftKey && event.code === 'KeyD') {
        window.nvm
          .openDesignTokenEditor()
          .then(setTokenState)
          .catch(() => {});
      }
    };
    window.addEventListener('keydown', open);
    if (new URLSearchParams(window.location.search).has('designTokens')) {
      window.nvm
        .openDesignTokenEditor()
        .then(setTokenState)
        .catch(() => {});
    }
    return () => window.removeEventListener('keydown', open);
  }, []);
  if (tokenState)
    return (
      <DesignTokenEditor
        initial={tokenState}
        onClose={() => {
          window.nvm
            .closeDesignTokenEditor()
            .finally(() => setTokenState(null));
        }}
      />
    );
  return extensionWindowId ? (
    <ExtensionWindowApp windowId={extensionWindowId} />
  ) : (
    <App />
  );
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>,
);
