import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'path';
import fs from 'fs';

function nodeDocsPlugin() {
  const docsPath = path.resolve(__dirname, '../NODES.md');
  return {
    name: 'goflow-node-docs',
    configureServer(server) {
      server.middlewares.use('/NODES.md', (_req, res) => {
        res.setHeader('Content-Type', 'text/markdown; charset=utf-8');
        res.end(fs.readFileSync(docsPath, 'utf-8'));
      });
    },
    generateBundle() {
      if (fs.existsSync(docsPath)) {
        this.emitFile({
          type: 'asset',
          fileName: 'NODES.md',
          source: fs.readFileSync(docsPath, 'utf-8'),
        });
      }
    },
  };
}

export default defineConfig({
  plugins: [vue(), nodeDocsPlugin()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
      '/webhook': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
