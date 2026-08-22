import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

const apiProxyTarget = process.env.CONTADINHO_DEV_API_URL ?? "http://localhost:8000";

export default defineConfig({
  cacheDir: ".vite-cache",
  plugins: [react()],
  server: {
    proxy: {
      "/api": apiProxyTarget,
      "/health": apiProxyTarget,
    },
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: "./src/test/setup.ts",
    restoreMocks: true,
    clearMocks: true,
    maxWorkers: 1,
    pool: "threads",
    testTimeout: 15_000,
  },
});
