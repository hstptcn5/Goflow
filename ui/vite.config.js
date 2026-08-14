import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'path';
import fs from 'fs';

const normalizeLineEndings = (value) => value.replace(/\r\n?/g, '\n');

function normalizedTextInputsPlugin() {
  const sourceRoot = `${path.resolve(__dirname, 'src')}${path.sep}`;
  return {
    name: 'goflow-normalized-text-inputs',
    enforce: 'pre',
    load(id) {
      if (id.includes('?') || !id.startsWith(sourceRoot)) return null;
      if (!/\.(?:css|js|json|ts|vue)$/.test(id)) return null;
      return normalizeLineEndings(fs.readFileSync(id, 'utf-8'));
    },
    transform(code, id) {
      const filePath = id.split('?', 1)[0];
      if (!filePath.startsWith(sourceRoot)) return null;
      return { code: normalizeLineEndings(code), map: null };
    },
    transformIndexHtml(html) {
      return normalizeLineEndings(html);
    },
  };
}

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
          source: normalizeLineEndings(fs.readFileSync(docsPath, 'utf-8')),
        });
      }
    },
  };
}

export default defineConfig({
  plugins: [
    normalizedTextInputsPlugin(),
    vue({ features: { componentIdGenerator: 'filepath' } }),
    nodeDocsPlugin(),
  ],
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
