import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

function vendorChunk(id) {
  // Shared CommonJS helpers must not create a dependency back to the app entry.
  if (id.includes("commonjsHelpers")) return "vendor-runtime";
  const path = id.replaceAll("\\", "/").split("/node_modules/")[1];
  if (!path) return;
  const name = path.startsWith("@")
    ? path.split("/").slice(0, 2).join("/")
    : path.split("/")[0];

  if (
    ["react", "react-dom", "scheduler", "use-sync-external-store"].includes(
      name,
    )
  )
    return "vendor-react";
  if (name === "three") return "vendor-three";
  if (name === "xlsx") return "vendor-spreadsheets";
  if (
    name === "pdf-lib" ||
    name.startsWith("@pdf-lib/") ||
    ["pako", "tslib"].includes(name)
  )
    return "vendor-pdf";
  if (
    name.startsWith("prosemirror-") ||
    ["orderedmap", "rope-sequence", "w3c-keyname"].includes(name)
  )
    return "vendor-prosemirror";
  if (["yjs", "lib0", "y-protocols", "@tiptap/y-tiptap"].includes(name))
    return "vendor-collaboration";
  if (
    name.startsWith("@tiptap/") ||
    ["marked", "linkifyjs", "fast-equals"].includes(name)
  )
    return "vendor-editor";
  if (name === "fflate") return "vendor-archives";
  if (["delaunator", "robust-predicates"].includes(name))
    return "vendor-geometry";
  // Let Rollup place other dependencies at their actual point of use instead
  // of pulling every optional feature into one eagerly loaded vendor bundle.
}

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": { target: "http://127.0.0.1:8124", ws: true },
      "/auth": "http://127.0.0.1:8124",
    },
  },
  build: {
    outDir: "dist",
    chunkSizeWarningLimit: 600,
    rollupOptions: {
      output: {
        onlyExplicitManualChunks: true,
        manualChunks: vendorChunk,
      },
    },
  },
});
