import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': 'http://localhost:7777',
      '/metrics': 'http://localhost:7777'
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true
  }
})
