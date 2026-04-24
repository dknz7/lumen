import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

export default defineConfig({
  plugins: [solid()],
  build: {
    outDir: "../internal/server/web/dist",
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    // Dev-mode only — proxy /api to the Go backend running on 7832.
    proxy: {
      "/api": "http://127.0.0.1:7832",
    },
  },
});
