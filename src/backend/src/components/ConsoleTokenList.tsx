import { useEffect, useState } from 'react';
import { dashboardFetch } from '../lib/dashboard-fetch';
import { watchDashboard } from '../lib/dashboard-refresh';
import { Button } from './ui/button';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from './ui/alert-dialog';

type Token = {
  id: string;
  prefix: string;
  name: string;
  created: string;
  lastUsed: string;
};

export function ConsoleTokenList({ tokens }: { tokens: Token[] }) {
  const [currentTokens, setCurrentTokens] = useState(tokens);
  const [refreshFailed, setRefreshFailed] = useState(false);
  const [pending, setPending] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [removed, setRemoved] = useState<string[]>([]);
  const [openToken, setOpenToken] = useState<string | null>(null);
  const remaining = currentTokens.filter((token) => !removed.includes(token.id));

  useEffect(function watchTokens() {
    let active = true;
    const stop = watchDashboard(async () => {
      if (openToken) return;
      try {
        const result = await dashboardFetch<{ tokens: Array<{ id: string; prefix: string; name: string; createdAt: string; lastUsedAt: string | null }> }>('/api/tokens');
        if (!active) return;
        setCurrentTokens(result.tokens.map((token) => ({
          id: token.id, prefix: token.prefix, name: token.name,
          created: new Date(token.createdAt).toLocaleString(),
          lastUsed: token.lastUsedAt ? `last used ${new Date(token.lastUsedAt).toLocaleString()}` : 'never used',
        })));
        setRefreshFailed(false);
      } catch { if (active) setRefreshFailed(true); }
    });
    return () => { active = false; stop(); };
  }, [openToken]);

  useEffect(
    function restoreFocusAfterRevocation() {
      if (removed.length > 0)
        document.getElementById('tokens-heading')?.focus();
    },
    [removed.length],
  );

  async function revoke(id: string) {
    setPending(id);
    setError('');
    try {
      const response = await fetch(`/api/tokens/${encodeURIComponent(id)}`, {
        method: 'DELETE',
      });
      if (!response.ok)
        throw new Error('Could not revoke this token; try again.');
      setRemoved((current) => [...current, id]);
      setOpenToken(null);
    } catch {
      setError('Could not revoke this token; try again.');
    } finally {
      setPending(null);
    }
  }

  return (
    <>
      {refreshFailed && <p className="status status--err" role="status">Could not refresh tokens; showing the last list.</p>}
      <ul className="token-list">
        {remaining.map(function renderToken(token) {
          return (
            <li className="token-list-item" key={token.id}>
              <div className="token-details">
                <strong className="font-medium">{token.name}</strong>{' '}
                <code className="ml-2 text-muted-foreground">
                  {token.prefix}…
                </code>
                <p>
                  Created {token.created} · {token.lastUsed}
                </p>
              </div>
              <AlertDialog
                open={openToken === token.id}
                onOpenChange={function toggleOpen(open) {
                  setOpenToken(open ? token.id : null);
                  setError('');
                }}
              >
                <AlertDialogTrigger asChild>
                  <Button variant="outline" size="sm">
                    Revoke
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>Revoke {token.name}?</AlertDialogTitle>
                    <AlertDialogDescription>
                      Anything using this token will lose access immediately;
                      this cannot be undone.
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  {error && (
                    <p className="status status--err" role="alert">
                      {error}
                    </p>
                  )}
                  <AlertDialogFooter>
                    <AlertDialogCancel disabled={pending !== null}>
                      Keep token
                    </AlertDialogCancel>
                    <Button
                      variant="destructive"
                      disabled={pending !== null}
                      onClick={function confirmRevoke() {
                        void revoke(token.id);
                      }}
                    >
                      {pending === token.id ? 'Revoking…' : 'Revoke token'}
                    </Button>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </li>
          );
        })}
        {remaining.length === 0 && (
          <li className="text-sm text-muted-foreground">No API tokens.</li>
        )}
      </ul>
      <p className="sr-only" role="status">
        {removed.length > 0
          ? `${removed.length} token${removed.length === 1 ? '' : 's'} revoked.`
          : ''}
      </p>
    </>
  );
}
