import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { execSync } from 'child_process';

// Get build info from environment variables (set by claude-hub.sh or docker-compose args)
// Falls back to git command for dev server mode
function getGitCommit(): string {
  if (process.env.VITE_GIT_COMMIT) return process.env.VITE_GIT_COMMIT;
  try {
    return execSync('git rev-parse --short HEAD', { encoding: 'utf-8' }).trim();
  } catch {
    return 'unknown';
  }
}

const gitCommit = getGitCommit();
const buildTime = process.env.VITE_BUILD_TIME || new Date().toISOString();

export default defineConfig({
  plugins: [react()],
  appType: 'spa',
  define: {
    __APP_VERSION__: JSON.stringify('1.0.0'),
    __GIT_COMMIT__: JSON.stringify(gitCommit),
    __BUILD_TIME__: JSON.stringify(buildTime),
  },
  server: {
    host: true,  // Listen on all interfaces (0.0.0.0) for LAN access
    port: 3000,
    headers: {
      'Cache-Control': 'no-store, no-cache, must-revalidate, proxy-revalidate',
      'Pragma': 'no-cache',
      'Expires': '0',
      'CDN-Cache-Control': 'no-store',
      'Cloudflare-CDN-Cache-Control': 'no-store',
    },
    allowedHosts: [
      '8f32963897b2.rgzn71nbigob.dev',
    ],
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/agent-economy-api': {
        target: 'http://localhost:8402',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/agent-economy-api/, ''),
      },
    },
  },
});
