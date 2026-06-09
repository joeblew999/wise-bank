# wise-bank — design & decisions

The single doc to revisit for how this repo drives the Wise API and why it's built
the way it is.

## Approach: drive the published spec, don't hand-write

Wise is a **REST** API with a full, official **OpenAPI 3.1** spec. We vendor that spec
and point off-the-shelf tools at it, so the *entire* API is usable with zero
per-endpoint code:

- **CLI** — `mise run api` (restish): every endpoint is a command (~205).
- **MCP** — `mise run mcp:openapi` (merzzzl/openapi-mcp-server): 239 tools for agents.
- **Web GUI** — `mise run serve:web`: a hand-written Go server (the one place we keep
  hand-written Go, because there's no spec-driven web dashboard).

## Source of truth (local — never re-scrape)

| File | What |
|------|------|
| [`reference/wise-openapi.yaml`](reference/wise-openapi.yaml) | Official Wise OpenAPI 3.1 bundle, verbatim. 198 paths / 239 ops / 131 schemas. |
| [`reference/wise-endpoints.txt`](reference/wise-endpoints.txt) | Greppable `METHOD path — summary` index. |
| [`reference/wise-openapi-3.0.yaml`](reference/) | Normalized 3.0 form (gitignored, generated). |
| [`../API.md`](../API.md) | Go SDK coverage matrix (what the library implements). |

Refresh: `mise run spec:fetch` (re-download + index). Official URL:
`https://docs.wise.com/_bundle/api-reference/index.yaml`.

## The normalize pipeline (why 3.0 exists)

Wise ships **OpenAPI 3.1**, but the Go tools we use (`restish`, the MCP proxy — both
on `kin-openapi`) are **3.0-only and strict**. `mise run spec:normalize`
([`../scripts/normalize-spec.cjs`](../scripts/normalize-spec.cjs)) downconverts and
fixes the spec so they accept it:

1. 3.1 → 3.0 downconvert (`@apiture/openapi-down-convert`)
2. drop numeric `exclusiveMinimum`/`Maximum`
3. strip `example`/`examples` (Wise's contain `null` for non-nullable fields → rejected)
4. drop 3.1-only keywords (`dependentRequired`, `$id`, …) + `const`→`enum`
5. set `info.version` (Wise ships it empty)

restish/MCP need the 3.0 output; `api:setup` and `mcp:openapi` depend on it.

## What's NOT possible via the API (PSD2)

On **personal** Wise accounts, Wise removed request-signing/SCA (PSD2). Per Wise's
public-keys page:

> we no longer support signing API requests to complete strong customer authentication
> on personal Wise accounts. You can no longer retrieve account statements or fund
> payments using this method. It is still possible to create draft transfers … and fund
> them from your multi-currency account using our website or mobile apps.

So on a personal account the API gives **reads + draft transfers**; **statements** and
**funding/conversion** are not API-accessible (do them in the web/app). There's no
public key to register. Business accounts may differ — TBD. (restish/MCP can't sign
anyway.)

## Verify

`mise run smoke` — tools, spec, restish config (offline, no prompts).
`mise run smoke:auth` — also live prod+sandbox auth (reads keychain; opt-in).

## Decision log (so we can always revisit)

Worked out 2026-06-09.

- **Considered a Rust ConnectRPC service** (mirroring `joeblew999/google_maps` and
  `cf-do-locator`: proto → connectrpc-build + native/worker features + TS client).
  **Rejected** — that re-exposes Wise as our own typed RPC, which we don't need just to
  *use* Wise. restish + openapi-mcp already give the whole API from the spec. The proto
  + Rust design were removed.
- **No OpenAPI→client codegen** — restish is the CLI (runtime over the spec). Also:
  OpenAPI→proto is unsupported/lossy (35 `oneOf` + 26 `discriminator`); the ecosystem
  direction is proto→OpenAPI.
- **Removed the hand-written Go CLI (`wise-cli`) and Go MCP server (`wise-mcp`)** —
  superseded by `api:*` and `mcp:openapi`. Kept the Go library + `commands/` because the
  **web GUI** (`wise-server`) uses them.
- **Spec is the source of truth**, vendored locally; `API.md` is the older Go matrix.

**Reference repos / sources:** `joeblew999/google_maps` (`crates/connectrpc`),
`cf-do-locator`; official spec URL found via `soenneker/soenneker.wise.openapiclient`
(regenerates from the same bundle). Tooling: restish (`rest-sh/restish`),
`merzzzl/openapi-mcp-server`, `@apiture/openapi-down-convert`.
