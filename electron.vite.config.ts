import react from '@vitejs/plugin-react';
import { defineConfig } from 'electron-vite';

// biome-ignore lint/style/noDefaultExport: Electron-vite requires default export
export default defineConfig({
  main: {
    build: {
      outDir: 'dist/main',
      lib: {
        entry: 'src/electron/main.ts',
        formats: ['es'],
        fileName: () => 'index.js',
      },
    },
  },
  preload: {
    build: {
      outDir: 'dist/preload',
      lib: {
        entry: 'src/electron/preload.ts',
        formats: ['cjs'],
        fileName: () => 'preload.js',
      },
    },
  },
  renderer: {
    root: '.',
    base: './',
    plugins: [react()],
    server: {
      host: '127.0.0.1',
      port: 5173,
      strictPort: true,
    },
    build: {
      outDir: 'dist/renderer',
      emptyOutDir: true,
      target: 'esnext',
      sourcemap: false,
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
