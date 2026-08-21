import { parsePublicOrigin } from '../../../app/shared/public-origin';
import { env } from './env';

export function localDevelopmentOrigin(origin: string) {
  const vercelEnvironment = env('VERCEL_ENV');
  if (
    vercelEnvironment
      ? vercelEnvironment !== 'development'
      : env('NODE_ENV') === 'production'
  )
    return null;
  try {
    return parsePublicOrigin(origin, 'local');
  } catch {
    return null;
  }
}
