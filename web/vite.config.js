import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const controller = process.env.SDWAN_DEV_CONTROLLER || 'http://127.0.0.1:18080'

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/admin': controller,
      '/api': controller,
    },
  },
})
