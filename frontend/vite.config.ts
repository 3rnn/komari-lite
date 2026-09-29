import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { visualizer } from "rollup-plugin-visualizer";
import { VitePWA } from "vite-plugin-pwa";

// https://vite.dev/config/
import type { UserConfig } from "vite";
import * as fs from "fs";
import * as path from "path";
import { fileURLToPath } from "url";
import dotenv from "dotenv";

const configDir = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig(({ mode }) => {
  const buildTime = new Date().toISOString();
  const systemUiBuild = mode !== "development" && process.env.VITE_SYSTEM_UI_BUILD !== "0";

  // Production builds are embedded into Komari. An explicit opt-out is required
  // for a standalone root-path build.
  const base: string = process.env.VITE_BASE_URL
    ? process.env.VITE_BASE_URL
    : systemUiBuild
      ? "/system-assets/"
      : "/";
  const baseConfig: UserConfig = {
    base: base,
    // Preserve non-Latin input aliases as escapes in generated JavaScript.
    esbuild: { charset: "ascii" },
    plugins: [
      react(),
      tailwindcss(),
      ...(systemUiBuild ? [] : [VitePWA({
        registerType: "autoUpdate",
        includeManifestIcons: false,
        manifest: {
          name: "Komari Lite",
          short_name: "Komari Lite",
          description: "A simple server monitor tool",
          theme_color: "#2563eb",
          background_color: "#ffffff",
          display: "standalone",
          scope: base,
          start_url: base,
          icons: [
            {
              src: `${base}assets/pwa-icon.png`,
              sizes: "192x192",
              type: "image/png",
              purpose: "maskable any",
            },
            {
              src: `${base}assets/pwa-icon.png`,
              sizes: "512x512",
              type: "image/png",
              purpose: "maskable any",
            },
          ],
        },
        workbox: {
          cleanupOutdatedCaches: true,
          // HTML is rendered by Komari so it can inject the current site
          // title and custom Head/Body content. Precaching the build-time
          // index would bypass that server-side rendering.
          globPatterns: ["**/*.{js,css,ico,png,svg}"],
          // The public document is selected by Komari at request time. A
          // cached SPA fallback would keep serving the previous theme after
          // an administrator switches themes.
          navigateFallback: null,
          runtimeCaching: [
            {
              urlPattern: /^https:\/\/api\./i,
              handler: "NetworkFirst",
              options: {
                cacheName: "api-cache",
                expiration: {
                  maxEntries: 10,
                  maxAgeSeconds: 60 * 60 * 24 * 365, // <== 365 days
                },
                cacheableResponse: {
                  statuses: [0, 200],
                },
              },
            },
          ],
        },
      })]),
      ...(process.env.ANALYZE === "1"
        ? [visualizer({
            open: false,
            filename: "bundle-analysis.html",
            gzipSize: true,
            brotliSize: true,
          })]
        : []),
    ],
    define: {
      __BUILD_TIME__: JSON.stringify(buildTime),
    },
    resolve: {
      alias: [
        { find: "@", replacement: path.resolve(configDir, "./src") },
      ],
    },
    build: {
      assetsDir: "assets",
      outDir: "dist",
      chunkSizeWarningLimit: 800,
      rollupOptions: {
        output: {
          // go embed ignore files start with '_'
          chunkFileNames: "assets/chunk-[name]-[hash].js",
          entryFileNames: "assets/entry-[name]-[hash].js",
          // Do not use manualChunks, use React.lazy() and <Suspense> instead
        }
      },
    },
  };

  if (mode === "development") {
    const envPath = path.resolve(process.cwd(), ".env.development");
    if (fs.existsSync(envPath)) {
      const envConfig = dotenv.parse(fs.readFileSync(envPath));
      for (const k in envConfig) {
        process.env[k] = envConfig[k];
      }
    }
    const apiTarget = process.env.VITE_API_TARGET || "http://127.0.0.1:25774";
    process.env.VITE_API_TARGET = apiTarget;
    const apiOrigin = new URL(apiTarget).origin;
    const proxy = {
      "/api": {
        target: apiTarget,
        changeOrigin: true,
        rewriteWsOrigin: true,
        ws: true,
        headers: {
          Origin: apiOrigin,
        },
      },
      "/themes": {
        target: apiTarget,
        changeOrigin: true,
      },
    };
    baseConfig.server = {
      proxy,
    };
    baseConfig.preview = {
      proxy: {
        ...proxy,
      },
    };
  }

  return baseConfig;
});
