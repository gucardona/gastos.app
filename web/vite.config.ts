import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

// In development Go runs on :8000 and Vite on :5173; the proxy keeps everything on one origin (cookies work).
export default defineConfig({
  plugins: [react()],
  server: { proxy: { "/api": "http://127.0.0.1:8000" } },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: ["src/test/setup.ts"],
  },
});
