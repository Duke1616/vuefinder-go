import { fileURLToPath, URL } from "node:url";
import { resolve } from "node:path";

import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

// https://vite.dev/config/
const __dirname = fileURLToPath(new URL(".", import.meta.url));

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      // Fix for vuefinder locales import - map to actual file paths
      "vuefinder/dist/locales": resolve(__dirname, "node_modules/vuefinder/dist/locales"),
    },
    dedupe: ["vue"],
  },
  optimizeDeps: {
    include: ["vuefinder"],
  },
});
