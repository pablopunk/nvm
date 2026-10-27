import { buttonVariants, Button } from './ui/button';

type Props = { email: string; admin: boolean; current: 'profile' | 'admin' };

export function ConsoleNavigation({ email, admin, current }: Props) {
  return (
    <nav className="console-navigation" aria-label="Account navigation">
      <a className="console-navigation__brand" href="/">Nevermind</a>
      <a className={buttonVariants({ variant: current === 'profile' ? 'secondary' : 'ghost', size: 'sm' })} href="/profile" aria-current={current === 'profile' ? 'page' : undefined}>Profile</a>
      {admin && <a className={buttonVariants({ variant: current === 'admin' ? 'secondary' : 'ghost', size: 'sm' })} href="/admin" aria-current={current === 'admin' ? 'page' : undefined}>Admin</a>}
      <span className="console-navigation__account">{email}</span>
      <form method="post" action="/api/auth/signout"><Button variant="outline" size="sm" type="submit">Sign out</Button></form>
    </nav>
  );
}
