import { useEffect, useMemo, useState } from 'react';
import {
  DESIGN_TOKEN_DEFAULTS,
  type DesignTokenName,
  type DesignTokenOverrides,
  type DesignTokenValues,
} from './design-tokens';
import type { DesignTokenState } from './preload-api';

export type DesignTokenEditorApi = {
  setDesignTokens: (
    overrides: DesignTokenOverrides,
  ) => Promise<DesignTokenState>;
  resetDesignTokens: () => Promise<DesignTokenState>;
};

const GROUPS = [
  [
    'Spacing',
    ['--window-blur-margin', '--search-row-height', '--palette-stack-gap'],
  ],
  [
    'Radii',
    Object.keys(DESIGN_TOKEN_DEFAULTS).filter((name) =>
      name.startsWith('--radius-'),
    ),
  ],
  [
    'Type',
    [
      ...Object.keys(DESIGN_TOKEN_DEFAULTS).filter((name) =>
        name.startsWith('--fs-'),
      ),
      '--font-family-ui',
    ],
  ],
  [
    'Surfaces',
    Object.keys(DESIGN_TOKEN_DEFAULTS).filter((name) =>
      name.startsWith('--surface-'),
    ),
  ],
  [
    'Borders',
    Object.keys(DESIGN_TOKEN_DEFAULTS).filter((name) =>
      name.startsWith('--border-'),
    ),
  ],
  [
    'Text',
    Object.keys(DESIGN_TOKEN_DEFAULTS).filter(
      (name) => name.startsWith('--text-') || name === '--link',
    ),
  ],
  [
    'Accent',
    Object.keys(DESIGN_TOKEN_DEFAULTS).filter(
      (name) => name.startsWith('--accent') || name.startsWith('--danger-'),
    ),
  ],
] as const;

function applyTokens(values: DesignTokenValues) {
  for (const [name, value] of Object.entries(values)) {
    document.documentElement.style.setProperty(name, value);
  }
}

function Preview({ tab }: { tab: string }) {
  if (tab === 'subview') {
    return (
      <div className="tokenFixture tokenSubview">
        <header>
          <button type="button">‹</button>
          <strong>Project details</strong>
          <kbd>⌘ ↵</kbd>
        </header>
        <section>
          <span className="tokenEyebrow">OVERVIEW</span>
          <h2>Ship the design token editor</h2>
          <p>
            Preview text, surfaces, borders, spacing, and typography together.
          </p>
        </section>
        <footer>
          <button type="button">Cancel</button>
          <button className="tokenPrimary" type="button">
            Save changes
          </button>
        </footer>
      </div>
    );
  }
  if (tab === 'command') {
    return (
      <div className="tokenFixture tokenCommandMenu">
        <div className="tokenSearch">
          Type a command… <kbd>⌘ K</kbd>
        </div>
        <span className="tokenEyebrow">SUGGESTED</span>
        {['Create new extension', 'Open settings', 'Check for updates'].map(
          (title, index) => (
            <div
              className={index === 0 ? 'tokenRow selected' : 'tokenRow'}
              key={title}
            >
              <span className="tokenIcon">{index + 1}</span>
              <span>
                <strong>{title}</strong>
                <small>Nevermind command</small>
              </span>
              <kbd>↵</kbd>
            </div>
          ),
        )}
      </div>
    );
  }
  return (
    <div className="tokenFixture tokenRootList">
      <div className="tokenSearch">
        Search apps and commands… <kbd>⌘ Space</kbd>
      </div>
      <span className="tokenEyebrow">ROOT ITEMS</span>
      {[
        'Ask Nevermind',
        'Clipboard history',
        'Browse extensions',
        'Settings',
      ].map((title, index) => (
        <div
          className={index === 0 ? 'tokenRow selected' : 'tokenRow'}
          key={title}
        >
          <span className="tokenIcon">{['✦', '▣', '◫', '⚙'][index]}</span>
          <span>
            <strong>{title}</strong>
            <small>
              {index === 0
                ? 'Chat with your AI assistant'
                : 'Open in Nevermind'}
            </small>
          </span>
          {index === 0 && <kbd>↵</kbd>}
        </div>
      ))}
    </div>
  );
}

export function DesignTokenEditor({
  initial,
  api = window.nvm,
  onClose,
}: {
  initial: DesignTokenState;
  api?: DesignTokenEditorApi;
  onClose?: () => void;
}) {
  const [state, setState] = useState(initial);
  const [drafts, setDrafts] = useState<DesignTokenValues>(initial.values);
  const [tab, setTab] = useState('root');
  const [error, setError] = useState('');
  const changed = useMemo(
    () => new Set(Object.keys(state.overrides)),
    [state.overrides],
  );

  useEffect(() => {
    applyTokens(state.values);
    return () => {
      for (const name of Object.keys(DESIGN_TOKEN_DEFAULTS))
        document.documentElement.style.removeProperty(name);
    };
  }, [state.values]);

  async function save(name: DesignTokenName, value: string) {
    const overrides: DesignTokenOverrides = {
      ...state.overrides,
      [name]: value,
    };
    try {
      const next = await api.setDesignTokens(overrides);
      setState(next);
      setDrafts(next.values);
      setError('');
    } catch (caught) {
      setError(
        caught instanceof Error ? caught.message : 'Invalid token value',
      );
    }
  }

  async function reset() {
    const next = await api.resetDesignTokens();
    setState(next);
    setDrafts(next.values);
    setError('');
  }

  return (
    <main className="tokenStudio" data-testid="design-token-editor">
      <aside className="tokenSidebar">
        <header>
          <div>
            <span className="tokenEyebrow">DEVELOPMENT</span>
            <h1>Design tokens</h1>
          </div>
          {onClose && (
            <button aria-label="Close editor" onClick={onClose} type="button">
              ×
            </button>
          )}
        </header>
        <div className="tokenSidebarActions">
          <button onClick={reset} type="button">
            Reset all
          </button>
          <button
            onClick={() =>
              navigator.clipboard.writeText(
                JSON.stringify(state.overrides, null, 2),
              )
            }
            type="button"
          >
            Copy JSON
          </button>
        </div>
        {error && (
          <p className="tokenError" role="alert">
            {error}
          </p>
        )}
        <div className="tokenControls">
          {GROUPS.map(([label, names]) => (
            <section key={label}>
              <h2>{label}</h2>
              {names.map((name) => {
                const token = name as DesignTokenName;
                const color =
                  !name.startsWith('--fs-') &&
                  !name.startsWith('--radius-') &&
                  ![
                    '--window-blur-margin',
                    '--search-row-height',
                    '--palette-stack-gap',
                    '--font-family-ui',
                  ].includes(name);
                return (
                  <label
                    className={changed.has(name) ? 'changed' : ''}
                    key={name}
                  >
                    <span>{name.replace('--', '')}</span>
                    <div>
                      {color && (
                        <input
                          aria-label={`${name} color`}
                          onChange={(event) =>
                            void save(token, event.target.value)
                          }
                          type="color"
                          value={
                            drafts[token].startsWith('#')
                              ? drafts[token].slice(0, 7)
                              : '#ffffff'
                          }
                        />
                      )}
                      <input
                        aria-label={name}
                        onBlur={(event) => void save(token, event.target.value)}
                        onChange={(event) =>
                          setDrafts((current) => ({
                            ...current,
                            [token]: event.target.value,
                          }))
                        }
                        onKeyDown={(event) => {
                          if (event.key === 'Enter') event.currentTarget.blur();
                        }}
                        value={drafts[token]}
                      />
                    </div>
                  </label>
                );
              })}
            </section>
          ))}
        </div>
      </aside>
      <section className="tokenCanvas">
        <header>
          <div>
            <span className="tokenEyebrow">LIVE PREVIEW</span>
            <h2>Nevermind UI</h2>
          </div>
          <nav>
            {[
              ['root', 'Root items'],
              ['subview', 'Sub-view'],
              ['command', 'Command K'],
            ].map(([id, label]) => (
              <button
                aria-pressed={tab === id}
                key={id}
                onClick={() => setTab(id)}
                type="button"
              >
                {label}
              </button>
            ))}
          </nav>
        </header>
        <div className="tokenStage">
          <Preview tab={tab} />
        </div>
        <footer>
          <span>
            {changed.size} override{changed.size === 1 ? '' : 's'} saved locally
          </span>
          <code>src/styles.css</code> remains unchanged
        </footer>
      </section>
    </main>
  );
}
