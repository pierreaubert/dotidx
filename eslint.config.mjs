import { defineConfig } from "eslint/config";
import globals from "globals";
import js from "@eslint/js";


export default defineConfig([
  { files: ["app/*.{js,mjs}"], plugins: {js}, languageOptions: { globals: globals.browser } }
]);