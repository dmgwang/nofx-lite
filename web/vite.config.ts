import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { execSync } from 'child_process'


let version = process.env.VITE_APP_VERSION
if (!version) {
    try {
        version = execSync('git rev-parse --short HEAD').toString().trim()
    } catch {
        version = 'dev'
    }
}

export default defineConfig(({ mode }) => {
    return {
        plugins: [react()],
        server: {
            host: '0.0.0.0',
            port: 3000,
            proxy: {
                '/api': {
                    target: 'http://localhost:8080',
                    changeOrigin: true,
                },
            },
        },
        define: {
            __APP_VERSION__: JSON.stringify(version),
        },
    }
})
