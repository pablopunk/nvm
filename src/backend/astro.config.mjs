// @ts-check
import { defineConfig } from 'astro/config';
import vercel from '@astrojs/vercel';
import sentry from '@sentry/astro';
import react from '@astrojs/react';
import tailwindcss from '@tailwindcss/vite';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const sentryDsn = process.env.SENTRY_DSN;

/**
 * Regenerates the public docs data from the desktop app's sources on every dev,
 * check, and build run. Runs here instead of an npm pre-hook because Vercel's
 * Astro preset invokes `astro build` directly. It never fails the build.
 */
const generatedDocs = {
  name: 'generated-docs',
  hooks: {
    'astro:config:setup': function generateDocsData() {
      try {
        execFileSync(
          process.execPath,
          ['--import', 'tsx', fileURLToPath(new URL('./scripts/generate-docs-data.ts', import.meta.url))],
          { stdio: 'inherit' },
        );
      } catch (error) {
        console.warn('[docs] Generation failed; docs pages will render without generated sections.', error);
      }
    },
  },
};

export default defineConfig({
  output: 'server',
  adapter: vercel(),
  site: 'https://www.nvm.fyi',
  vite: { plugins: [tailwindcss()] },
  integrations: [
    generatedDocs,
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
