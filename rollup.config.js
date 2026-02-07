import commonjs from "@rollup/plugin-commonjs";
import json from "@rollup/plugin-json";
import { nodeResolve } from "@rollup/plugin-node-resolve";
import replace from "@rollup/plugin-replace";
import typescript from "@rollup/plugin-typescript";
import importAssets from "rollup-plugin-import-assets";
import deckyPlugin from "@decky/rollup";
import css from "rollup-plugin-import-css";

export default deckyPlugin({
  input: "./src/index.tsx",
  plugins: [
    commonjs(),
    nodeResolve(),
    typescript(),
    json(),
    css(),
    replace({
      preventAssignment: false,
      "process.env.NODE_ENV": JSON.stringify("production"),
    }),
    importAssets({
      publicPath: `http://127.0.0.1:1337/plugins/DeckyFileServer/`,
    }),
  ],
  context: "window",
  external: ["react", "react-dom"],
  output: {
    dir: "dist/index.js",
    globals: {
      react: "SP_REACT",
      "react-dom": "SP_REACTDOM",
    },
    format: "iife",
    exports: "default",
  },
});
