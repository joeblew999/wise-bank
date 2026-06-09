# Wise API reference (vendored locally)

So we don't re-scrape Wise's (truncating, JS-heavy) doc site every time.

| File | What |
|------|------|
| `wise-openapi.yaml` | The **official** Wise Platform OpenAPI 3.1 bundle, fetched verbatim from `https://docs.wise.com/_bundle/api-reference/index.yaml`. Self-contained (no external `$ref`s). ~1.3 MB, 198 paths. This is the authoritative endpoint + schema source. |
| `wise-endpoints.txt` | A flat, greppable index (METHOD path — summary) generated from the spec. Browse this first; open the YAML for full schemas. |

## Refresh both

```
mise run spec:fetch      # re-download the spec + regenerate the index
```

(Internally: `curl` the official URL → `scripts/gen-endpoint-index.nu`.)

## Notes

- This spec is **more current than [`../../API.md`](../../API.md)** (the hand-curated
  Go-coverage matrix). Where they disagree, trust the spec — e.g. recipients are
  `/v2/accounts` here, cards live under `/v4/spend/...`.
- Source URL discovered via `soenneker/soenneker.wise.runners.openapiclient`
  (a .NET generator that pulls the same bundle daily).
