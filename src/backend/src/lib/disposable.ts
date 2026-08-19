import domainsJson from '../data/disposable-domains.json' with { type: 'json' };

const DOMAINS = new Set((domainsJson as string[]).map((d) => d.toLowerCase()));

export function isDisposableEmail(email: string): boolean {
  const at = email.lastIndexOf('@');
  if (at < 0) return false;
  const domain = email.slice(at + 1).toLowerCase().trim();
  if (!domain) return false;
  if (DOMAINS.has(domain)) return true;
  const parts = domain.split('.');
  for (let i = 1; i < parts.length - 1; i++) {
    if (DOMAINS.has(parts.slice(i).join('.'))) return true;
  }
  return false;
}
