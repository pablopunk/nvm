import { useEffect, useState } from 'react';
import { dashboardQueryClient } from '../lib/dashboard-query-client';
import { useDashboardQuery } from '../lib/use-dashboard-query';
import { DashboardQueryError } from './dashboard/DashboardQueryError';
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

const TOKENS_URL = '/api/tokens';

type ApiToken = { id: string; prefix: string; name: string; createdAt: string; lastUsedAt: string | null };
type TokenList = { tokens: ApiToken[] };

function tokenLastUsedLabel(token: ApiToken) {
  return token.lastUsedAt ? `last used ${new Date(token.lastUsedAt).toLocaleString()}` : 'never used';
}

function TokenListSkeleton() {
  return <div aria-busy="true" aria-label="Loading API tokens"><span className="dashboard-skeleton" /><span className="dashboard-skeleton" /><span className="dashboard-skeleton" /></div>;
}

function forgetRevokedToken(id: string) {
  const client = dashboardQueryClient();
  client.setQueryData<TokenList>([TOKENS_URL], (current) => current && { tokens: current.tokens.filter((token) => token.id !== id) });
  void client.invalidateQueries({ queryKey: [TOKENS_URL] });
}

export function ConsoleTokenList() {
  const [pending, setPending] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [removed, setRemoved] = useState<string[]>([]);
  const [openToken, setOpenToken] = useState<string | null>(null);
  const query = useDashboardQuery<TokenList>(TOKENS_URL, { paused: openToken !== null });
  const remaining = query.data?.tokens ?? [];
  const refreshFailed = Boolean(query.data && query.isError);

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
      forgetRevokedToken(id);
      setOpenToken(null);
    } catch {
      setError('Could not revoke this token; try again.');
    } finally {
      setPending(null);
    }
  }

  if (!query.data) return query.isError ? <DashboardQueryError what="API tokens" retry={query.refetch} /> : <TokenListSkeleton />;

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
                  Created {new Date(token.createdAt).toLocaleString()} · {tokenLastUsedLabel(token)}
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
