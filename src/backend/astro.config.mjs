// @ts-check
import { defineConfig } from 'astro/config';
import vercel from '@astrojs/vercel';
import sentry from '@sentry/astro';
import react from '@astrojs/react';
import tailwindcss from '@tailwindcss/vite';

const sentryDsn = process.env.SENTRY_DSN;

export default defineConfig({
  output: 'server',
  adapter: vercel(),
  site: 'https://www.nvm.fyi',
  vite: { plugins: [tailwindcss()] },
  integrations: [
    react(),
    ...(sentryDsn
      ? [
          sentry({
            dsn: sentryDsn,
            release: process.env.VERCEL_GIT_COMMIT_SHA,
            environment: process.env.VERCEL_ENV ?? 'development',
            serverInitPath: './src/sentry.server.config.ts',
            sourceMapsUploadOptions: { telemetry: false },
          }),
        ]
      : []),
  ],
});
