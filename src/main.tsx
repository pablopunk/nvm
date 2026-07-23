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
    const unsubscribe = window.nvm.onOpenDesignTokenEditor(setTokenState);
    if (new URLSearchParams(window.location.search).has('designTokens')) {
      window.nvm
        .openDesignTokenEditor()
        .then(setTokenState)
        .catch(() => {});
    }
    return unsubscribe;
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
