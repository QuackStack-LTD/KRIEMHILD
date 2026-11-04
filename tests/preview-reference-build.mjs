// Reconstruct ONLY the former native-DPR preview policy for A/B measurements.
// This never edits production source or authoritative terrain.
import { build } from "vite";
let replaced = false;
await build({
  build: { outDir: ".cache/preview-reference-dist" },
  plugins: [
    {
      name: "preview-reference",
      enforce: "pre",
      transform(code, id) {
        if (!id.endsWith("/continuous/Studio.jsx")) return;
        const prop =
          "previewQuality={illustrating ? undefined : previewQuality}";
        if (!code.includes(prop))
          throw Error("Reference preview prop no longer matches source");
        replaced = true;
        return code.replace(prop, "previewQuality={undefined}");
      },
    },
  ],
});
if (!replaced) throw Error("Reference preview transform was not applied");
