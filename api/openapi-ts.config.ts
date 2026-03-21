/**
 * TypeScript SDK generation config for @hey-api/openapi-ts.
 *
 * Usage:
 *   npx @hey-api/openapi-ts
 *
 * This reads api/openapi.yaml and emits a fully-typed fetch client into
 * api/generated/ts/. The generated files should be committed and kept in sync
 * with the spec by the generate-sdk CI workflow.
 *
 * Install dev dependency (not checked into go.mod):
 *   npm install --save-dev @hey-api/openapi-ts
 */
import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  client: "@hey-api/client-fetch",
  input: "openapi.yaml",
  output: {
    path: "generated/ts",
    format: "prettier",
    lint: "eslint",
  },
});
