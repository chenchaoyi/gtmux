import { cloudflareTest } from "@cloudflare/vitest-plugin";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [cloudflareTest({
    wrangler: { configPath: "./wrangler.toml" },
    miniflare: {
      bindings: {
        REG_SECRET: "test-gate",
        CF_API_TOKEN: "test-api-token",
        CF_ACCOUNT_ID: "account",
        CF_ZONE_ID: "zone",
        ZONE_NAME: "example.dev",
        DIRECT_URL: "https://direct.example.dev",
        DIRECT_SYNC_TOKEN: "test-sync-token",
        DIRECT_ADMIN_TOKEN: "test-admin-token",
      },
    },
  })],
  test: { include: ["test/**/*.spec.ts"] },
});
