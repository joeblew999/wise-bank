// Normalize the downconverted Wise spec so strict OpenAPI-3.0 loaders (e.g. Go's
// kin-openapi, used by many off-the-shelf OpenAPI->MCP / proxy tools) accept it.
//
// Input/output: docs/reference/wise-openapi-3.0.yaml (in place).
// Run via:  npx -y -p js-yaml@4 node scripts/normalize-spec.cjs
//
// Fixes applied:
//  1. numeric exclusiveMinimum/Maximum (3.1 style) -> 3.0 boolean form + bound
//  2. strip example/examples everywhere (Wise's examples contain nulls for
//     non-nullable fields, which strict validators reject; examples are doc-only)
const fs = require("fs");
const yaml = require("js-yaml");

const file = "docs/reference/wise-openapi-3.0.yaml";
const doc = yaml.load(fs.readFileSync(file, "utf8"));

// JSON-Schema-2020 / OpenAPI-3.1 keywords that strict 3.0 loaders reject as
// "extra sibling fields". Safe to drop for a runtime proxy (validation hints only).
const DROP_31 = new Set([
  "dependentRequired", "dependentSchemas", "prefixItems", "unevaluatedProperties",
  "unevaluatedItems", "contentMediaType", "contentEncoding", "$schema", "$id",
  "$anchor", "patternProperties",
]);

let stripped = 0, excl = 0, dropped = 0;
function walk(o) {
  if (Array.isArray(o)) { o.forEach(walk); return; }
  if (!o || typeof o !== "object") return;
  if ("example" in o) { delete o.example; stripped++; }
  if ("examples" in o) { delete o.examples; stripped++; }
  for (const k of DROP_31) { if (k in o) { delete o[k]; dropped++; } }
  // 3.1 `const: x` -> 3.0 `enum: [x]`
  if ("const" in o) { o.enum = [o.const]; delete o.const; dropped++; }
  // exclusiveMinimum/Maximum is a no-win across this tool's two layers: the 3.0 loader
  // (kin-openapi) wants a bool, the MCP tool-schema builder wants a number. Drop it
  // entirely, preserving the numeric bound as plain minimum/maximum.
  for (const k of ["exclusiveMinimum", "exclusiveMaximum"]) {
    if (k in o) {
      if (typeof o[k] === "number") o[k === "exclusiveMinimum" ? "minimum" : "maximum"] = o[k];
      delete o[k];
      excl++;
    }
  }
  for (const key of Object.keys(o)) walk(o[key]);
}
walk(doc);

doc.openapi = "3.0.3";
// kin-openapi requires a non-empty info.version (Wise ships version: '').
if (!doc.info) doc.info = {};
if (!doc.info.version) doc.info.version = "1.0.0";

const header = [
  "# GENERATED — do not edit by hand. Regenerate: mise run wise:spec:normalize",
  "# Source : docs/reference/wise-openapi.yaml (official Wise OpenAPI 3.1 bundle,",
  "#          https://docs.wise.com/_bundle/api-reference/index.yaml)",
  "# Pipeline: @apiture/openapi-down-convert (3.1->3.0) + scripts/normalize-spec.cjs",
  "#          (fix exclusiveMinimum, strip examples, drop 3.1-only keywords, set version)",
  "# Why     : strict OpenAPI-3.0 loaders (e.g. Go kin-openapi) reject Wise's raw 3.1 spec.",
  "",
].join("\n");
fs.writeFileSync(file, header + yaml.dump(doc, { lineWidth: -1, noRefs: true }));
console.log(`normalized: stripped ${stripped} example nodes, fixed ${excl} exclusive bounds, dropped ${dropped} 3.1-only keywords`);
