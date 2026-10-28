// @ts-check
import { defineConfig } from 'astro/config';
import vercel from '@astrojs/vercel';
import sentry from '@sentry/astro';
import react from '@astrojs/react';
import tailwindcss from '@tailwindcss/vite';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const sentryDsn = process.env.SENTRY_DSN;
const repoRoot = fileURLToPath(new URL('../..', import.meta.url));
const FULL_COMMIT_SHA_PATTERN = /^[0-9a-f]{40}$/;

/**
 * Resolves the source commit baked into each server bundle. Vercel CLI deploys are not
 * documented to expose VERCEL_GIT_COMMIT_SHA, so the deploy workflow passes the checked-out
 * SHA as NVM_BUILD_COMMIT_SHA. The nightly gate requires the resulting identity.
 */
function resolveBuildCommitSha() {
  const commit = process.env.NVM_BUILD_COMMIT_SHA?.trim() || null;
  if (commit && !FULL_COMMIT_SHA_PATTERN.test(commit)) {
    throw new Error('NVM_BUILD_COMMIT_SHA must be a full lowercase 40-character commit SHA');
  }
  return commit;
}

const buildCommitSha = resolveBuildCommitSha();

/**
 * Exposes the desktop app's built-in palette items to the landing page on every
 * dev, check, and build run. Runs here instead of an npm pre-hook because
 * Vercel's Astro preset invokes `astro build` directly. It never fails the build.
 */
const generatedPaletteItems = {
  name: 'generated-palette-items',
  hooks: {
    'astro:config:setup': function generatePaletteItems() {
      try {
        execFileSync(
          process.execPath,
          ['--import', 'tsx', fileURLToPath(new URL('./scripts/generate-palette-items.ts', import.meta.url))],
          { stdio: 'inherit' },
        );
      } catch (error) {
        console.warn('[palette] Generation failed; the landing palette uses sample content only.', error);
      }
    },
  },
};

export default defineConfig({
  output: 'server',
  adapter: vercel(),
  site: 'https://www.nvm.fyi',
  vite: {
    plugins: [tailwindcss()],
    // The landing palette imports the desktop app's palette styles and ranking.
    server: { fs: { allow: [repoRoot] } },
    define: { __NVM_BUILD_COMMIT_SHA__: JSON.stringify(buildCommitSha) },
  },
  integrations: [
    generatedPaletteItems,
    react(),
    ...(sentryDsn
      ? [
          sentry({
            dsn: sentryDsn,
            release: buildCommitSha ?? process.env.VERCEL_GIT_COMMIT_SHA,
            environment: process.env.VERCEL_ENV ?? 'development',
            serverInitPath: './src/sentry.server.config.ts',
            sourceMapsUploadOptions: { telemetry: false },
          }),
        ]
      : []),
  ],
});
