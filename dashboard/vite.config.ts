import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

const devPort = Number.parseInt(process.env.DASHBOARD_DEV_PORT ?? '5173', 10);
const apiTarget = process.env.DASHBOARD_API_TARGET ?? 'http://127.0.0.1:7734';

export default defineConfig({
  plugins: [svelte()],
  base: './',
  server: {
    host: '127.0.0.1',
    port: devPort,
    strictPort: true,
    proxy: {
      '/api': {
        target: apiTarget,
        changeOrigin: false,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: true,
  },
});
