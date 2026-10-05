import { resolve } from 'node:path';
import { createRequire } from 'node:module';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'electron-vite';
const { desktopBuildIdentity } = createRequire(import.meta.url)(
  './scripts/desktop-build-identity.cjs',
);

const buildDefinitions = {
  __NEVERMIND_BUILD__: JSON.stringify(desktopBuildIdentity()),
};

// biome-ignore lint/style/noDefaultExport: Electron-vite requires default export
export default defineConfig({
  main: {
    define: buildDefinitions,
    build: {
      sourcemap: 'hidden',
      outDir: 'dist/main',
      lib: {
        entry: 'src/app/electron/main.ts',
        formats: ['es'],
        fileName: () => 'main.js',
      },
    },
  },
  preload: {
    define: buildDefinitions,
    build: {
      sourcemap: 'hidden',
      outDir: 'dist/preload',
      lib: {
        entry: 'src/app/electron/preload.ts',
        formats: ['cjs'],
        fileName: () => 'preload.js',
      },
    },
  },
  renderer: {
    define: buildDefinitions,
    root: 'src/app/palette',
    base: './',
    plugins: [react()],
    server: {
      host: '127.0.0.1',
      port: 5173,
      strictPort: true,
    },
    build: {
      outDir: resolve('dist/renderer'),
      emptyOutDir: true,
      target: 'esnext',
      sourcemap: 'hidden',
      cssCodeSplit: false,
      rollupOptions: {
        input: 'index.html',
        output: {
          manualChunks(id) {
            if (!id.includes('node_modules')) {
              return;
            }
            if (
              id.includes('react-markdown') ||
              id.includes('remark-') ||
              id.includes('micromark') ||
              id.includes('mdast') ||
              id.includes('unist') ||
              id.includes('hast')
            ) {
              return 'markdown';
            }
            if (
              id.includes('/react/') ||
              id.includes('/react-dom/') ||
              id.includes('/scheduler/')
            ) {
              return 'react';
            }
            if (id.includes('/cmdk/')) {
              return 'cmdk';
            }
            if (id.includes('lucide-react')) {
              return 'icons';
            }
          },
        },
      },
    },
  },
});
