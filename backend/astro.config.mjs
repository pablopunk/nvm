// @ts-check
import { defineConfig } from 'astro/config';
import vercel from '@astrojs/vercel';
import sentry from '@sentry/astro';

const sentryDsn = process.env.SENTRY_DSN;

export default defineConfig({
  output: 'server',
  adapter: vercel(),
  site: 'https://www.nvm.fyi',
  integrations: sentryDsn
    ? [
        sentry({
          dsn: sentryDsn,
          release: process.env.VERCEL_GIT_COMMIT_SHA,
          environment: process.env.VERCEL_ENV ?? 'development',
          sourceMapsUploadOptions: { telemetry: false },
        }),
      ]
    : [],
});
