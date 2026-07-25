import React from 'react';
import ReactDOM from 'react-dom/client';
import {
  DesignTokenEditor,
  type DesignTokenEditorApi,
} from './design-token-editor';
import { DESIGN_TOKEN_DEFAULTS, resolveDesignTokens } from './design-tokens';
import type { DesignTokenState } from './preload-api';
import './styles.css';
import './design-token-editor.css';

const parameters = new URLSearchParams(window.location.hash.slice(1));
const apiUrl = parameters.get('api');
const apiToken = parameters.get('token');
const previewState: DesignTokenState = {
  enabled: true,
  defaults: { ...DESIGN_TOKEN_DEFAULTS },
  overrides: {},
  values: resolveDesignTokens({}),
};

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
  const [state, setState] = React.useState<DesignTokenState>(previewState);

  React.useEffect(() => {
    if (!(apiUrl && apiToken)) return;
    request<DesignTokenState>()
      .then(setState)
      .catch(() => {});
  }, []);

  return <DesignTokenEditor api={api} initial={state} />;
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserDesignTokenStudio />
  </React.StrictMode>,
);
