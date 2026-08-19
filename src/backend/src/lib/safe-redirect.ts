export function safeRelativeRedirectPath(value: string | null | undefined, fallback = '/dashboard') {
  const candidate = String(value || '').trim();
  if (!candidate) return fallback;
  if (!candidate.startsWith('/') || candidate.startsWith('//') || candidate.startsWith('/\\')) return fallback;

  try {
    const decoded = decodeURIComponent(candidate);
    if (decoded.startsWith('//') || decoded.startsWith('/\\')) return fallback;
    const parsed = new URL(candidate, 'https://nvm.local');
    if (parsed.origin !== 'https://nvm.local') return fallback;
    return `${parsed.pathname}${parsed.search}${parsed.hash}`;
  } catch {
    return fallback;
  }
}
