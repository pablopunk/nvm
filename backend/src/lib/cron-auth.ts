function isProduction(): boolean {
  const vercelEnv = process.env.VERCEL_ENV;
  if (vercelEnv) return vercelEnv === 'production';
  return process.env.NODE_ENV === 'production';
}

export function isAuthorizedCron(request: Request): boolean {
  const auth = request.headers.get('authorization');
  const secret = process.env.CRON_SECRET;
  if (!secret) {
    if (isProduction()) return false;
    return true;
  }
  return auth === `Bearer ${secret}`;
}
