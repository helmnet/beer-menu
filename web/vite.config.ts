import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    // Above the 5000 ms findBy/waitFor timeout in setup.ts, so a slow wait fails as itself.
    testTimeout: 20000,
  },
});
