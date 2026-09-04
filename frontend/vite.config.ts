import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
export default defineConfig({ plugins:[react()], server:{proxy:{'/api':'http://localhost:7001','/health':'http://localhost:7001','/metrics':'http://localhost:7001'}}})
