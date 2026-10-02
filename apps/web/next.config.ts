import type { NextConfig } from "next";
const config: NextConfig = {
  output: "export",
  distDir: "out-dev",
  trailingSlash: true,
  images: { unoptimized: true },
};
export default config;
