import React from 'react';
import ReactDOM from 'react-dom/client';
import {
  DesignTokenEditor,
  type DesignTokenEditorApi,
} from './design-token-editor';
import type { DesignTokenState } from './preload-api';
import './styles.css';
import './design-token-editor.css';

const parameters = new URLSearchParams(window.location.hash.slice(1));
const apiUrl = parameters.get('api');
const apiToken = parameters.get('token');

const api: DesignTokenEditorApi = {
  setDesignTokens(overrides) {
    return request<DesignTokenState>({
      method: 'PUT',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(overrides),
    });
  },
  resetDesignTokens() {
    return request<DesignTokenState>({ method: 'DELETE' });
  },
};

async function request<T>(init?: RequestInit): Promise<T> {
  if (!(apiUrl && apiToken))
    throw new Error('Open this studio from Nevermind.');
  const headers = new Headers(init?.headers);
  headers.set('x-nvm-token', apiToken);
  const response = await fetch(apiUrl, { ...init, headers });
  if (!response.ok) throw new Error(await response.text());
  return response.json() as Promise<T>;
}

function BrowserDesignTokenStudio() {
  const [state, setState] = React.useState<DesignTokenState | null>(null);
  const [error, setError] = React.useState('');

  React.useEffect(() => {
    request<DesignTokenState>()
      .then(setState)
      .catch((caught) =>
        setError(caught instanceof Error ? caught.message : 'Failed to load'),
      );
  }, []);

  if (error) return <main className="tokenStudioStatus">{error}</main>;
  if (!state) return <main className="tokenStudioStatus">Loading…</main>;
  return <DesignTokenEditor api={api} initial={state} />;
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserDesignTokenStudio />
  </React.StrictMode>,
);
