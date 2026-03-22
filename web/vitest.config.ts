import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import { resolve } from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: [
      // Route @xyflow/react to the stub mock so jsdom tests don't break
      {
        find: /^@xyflow\/react\/dist\/.*$/,
        replacement: resolve(__dirname, 'src/test/__mocks__/empty.css'),
      },
      {
        find: '@xyflow/react',
        replacement: resolve(__dirname, 'src/test/__mocks__/@xyflow/react.tsx'),
      },
    ],
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/test/setup.ts',
    css: true,
  },
})
