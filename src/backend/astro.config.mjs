// @ts-check
import { defineConfig } from 'astro/config';
import vercel from '@astrojs/vercel';
import react from '@astrojs/react';
import tailwindcss from '@tailwindcss/vite';
import { fileURLToPath } from 'node:url';

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

export default defineConfig({
  output: 'server',
  adapter: vercel(),
  site: 'https://www.nvm.fyi',
  vite: {
    plugins: [tailwindcss()],
    // The landing palette imports the shared palette styles and the extension API types from src/native.
    server: { fs: { allow: [repoRoot] } },
    define: { __NVM_BUILD_COMMIT_SHA__: JSON.stringify(buildCommitSha) },
  },
  integrations: [
    react(),
  ],
});
