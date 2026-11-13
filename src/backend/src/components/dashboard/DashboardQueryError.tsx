import { Button } from '../ui/button';

export function DashboardQueryError({ what, retry }: { what: string; retry: () => unknown }) {
  function retryQuery() { void retry(); }
  return <p role="alert">Could not load {what}. <Button variant="link" size="sm" type="button" onClick={retryQuery}>Retry</Button></p>;
}
