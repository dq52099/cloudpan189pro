import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const isProduction = mode === 'production'

  return {
    plugins: [vue()],
    esbuild: {
      drop: isProduction ? ['console', 'debugger'] : [],
    },
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    build: {
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (!id.includes('node_modules')) {
              return
            }

            if (id.includes('/@css-render/')) {
              return 'naive-core'
            }

            if (id.includes('/naive-ui/')) {
              if (
                id.includes('/naive-ui/es/_internal/') ||
                id.includes('/naive-ui/es/_mixins/') ||
                id.includes('/naive-ui/es/_styles/') ||
                id.includes('/naive-ui/es/_utils/') ||
                id.includes('/naive-ui/es/themes/')
              ) {
                return 'naive-core'
              }

              if (
                id.includes('/naive-ui/es/data-table/') ||
                id.includes('/naive-ui/es/pagination/') ||
                id.includes('/naive-ui/es/tree/') ||
                id.includes('/naive-ui/es/cascader/') ||
                id.includes('/naive-ui/es/select/') ||
                id.includes('/naive-ui/es/dropdown/')
              ) {
                return 'naive-data'
              }

              if (
                id.includes('/naive-ui/es/form/') ||
                id.includes('/naive-ui/es/input/') ||
                id.includes('/naive-ui/es/input-number/') ||
                id.includes('/naive-ui/es/date-picker/') ||
                id.includes('/naive-ui/es/radio/') ||
                id.includes('/naive-ui/es/checkbox/') ||
                id.includes('/naive-ui/es/switch/') ||
                id.includes('/naive-ui/es/slider/') ||
                id.includes('/naive-ui/es/dynamic-tags/')
              ) {
                return 'naive-form'
              }

              if (
                id.includes('/naive-ui/es/modal/') ||
                id.includes('/naive-ui/es/dialog/') ||
                id.includes('/naive-ui/es/drawer/') ||
                id.includes('/naive-ui/es/popover/') ||
                id.includes('/naive-ui/es/tooltip/') ||
                id.includes('/naive-ui/es/message/') ||
                id.includes('/naive-ui/es/notification/')
              ) {
                return 'naive-overlay'
              }

              if (
                id.includes('/naive-ui/es/layout/') ||
                id.includes('/naive-ui/es/grid/') ||
                id.includes('/naive-ui/es/space/') ||
                id.includes('/naive-ui/es/list/') ||
                id.includes('/naive-ui/es/breadcrumb/') ||
                id.includes('/naive-ui/es/menu/') ||
                id.includes('/naive-ui/es/tabs/') ||
                id.includes('/naive-ui/es/card/') ||
                id.includes('/naive-ui/es/descriptions/')
              ) {
                return 'naive-layout'
              }

              return 'naive-basic'
            }

            if (id.includes('/vue/') || id.includes('/vue-router/') || id.includes('/pinia/')) {
              return 'vue-vendor'
            }

            if (id.includes('/hls.js/') || id.includes('/plyr/')) {
              return 'media-vendor'
            }

            if (id.includes('/@vicons/')) {
              return 'icons'
            }

            return 'vendor'
          },
        },
      },
    },
    server: {
      host: '0.0.0.0',
      port: 5173,
      proxy: {
        '/api': {
          target: 'http://localhost:12395',
          changeOrigin: true,
        },
      },
    },
  }
})
