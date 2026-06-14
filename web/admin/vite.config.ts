import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  optimizeDeps: {
    exclude: ['@teggo/blocks'],
  },
  server: {
    port: 5173,
    proxy: {
      '/admin': { target: 'http://localhost:8080', changeOrigin: true },
      '/storefront': { target: 'http://localhost:8080', changeOrigin: true },
      '/signup': { target: 'http://localhost:8080', changeOrigin: true },
      '/demo': { target: 'http://localhost:8080', changeOrigin: true },
      // Invoice PDF capability URLs are served by the API.
      '/files': { target: 'http://localhost:8080', changeOrigin: true },
      // DAM blobs + signed transforms are served by the API.
      '/media': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
