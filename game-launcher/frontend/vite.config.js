import {defineConfig} from 'vite'
import {svelte} from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  plugins: [svelte()],
  server: {
    fs: {
      strict: false // Разрешаем Vite отдавать локальные картинки с других дисков
    }
  }
})