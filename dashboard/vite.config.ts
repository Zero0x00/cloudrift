import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // Dev: run `cloudrift dashboard --port 8080` (or match target) alongside `npm run dev`
      "/api": { target: "http://127.0.0.1:9090", changeOrigin: true }
    }
  }
});
