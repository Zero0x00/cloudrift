import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

// Default matches `cloudrift dashboard` (--port default 8080). Override in dashboard/.env.local:
//   VITE_API_PROXY_TARGET=http://127.0.0.1:9090
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const apiTarget = env.VITE_API_PROXY_TARGET || "http://127.0.0.1:8080";

  return {
    plugins: [react()],
    server: {
      proxy: {
        "/api": { target: apiTarget, changeOrigin: true }
      }
    }
  };
});
