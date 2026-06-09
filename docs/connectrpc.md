# Wise → ConnectRPC (Rust) — design, codegen & decision log

The single doc to revisit for: how we'd put a **Rust ConnectRPC service** in front of
the Wise API, how to **leverage Wise's published OpenAPI for code generation**, and
**the sources/decisions** behind it all. Pattern follows `joeblew999/google_maps`
(`crates/connectrpc`) and `cf-do-locator`.

---

## 0. Run it NOW (no code) — Wise REST → MCP ✅ PROVEN

Wise is a **REST** API with a full OpenAPI, so off-the-shelf tools run it as-is — no
ConnectRPC, no hand-written client. `mise run mcp:openapi` stands up the *entire* Wise
API as an MCP server (proxy) straight from the local spec; the MCP client supplies the
Wise token (forwarded as `Bearer`). Verified 2026-06-09: live `profileList` and
`rateGet` calls returned real data.

```
mise run mcp:openapi      # normalize spec (if needed) + serve MCP on :9090/wise/mcp
```

Config: [`../openapi-mcp.yaml`](../openapi-mcp.yaml) (tool = `merzzzl/openapi-mcp-server`,
Go). The `allow:` regex is `.*` → the **whole API: all 239 operations** become MCP
tools (verified: 239 registered). Narrow the regex to scope down (write/SCA ops are
exposed but funding/conversion need request signing).

### The real lesson: the spec needs **normalizing** for strict tools
Wise publishes **OpenAPI 3.1**, but many Go tools (here: `kin-openapi`) are **3.0-only
and strict**. Getting it to load took a reproducible pipeline (`mise run wise:spec:normalize`
→ [`../scripts/normalize-spec.cjs`](../scripts/normalize-spec.cjs)):
1. **3.1 → 3.0 downconvert** (`@apiture/openapi-down-convert`) — `exclusiveMinimum` bool vs number, etc.
2. **fix leftover numeric `exclusiveMinimum/Maximum`** the converter misses.
3. **strip `example`/`examples`** (2638 of them) — Wise's examples contain `null` for
   non-nullable fields, which strict validators reject. Examples are doc-only.
4. **drop 3.1-only keywords** (`dependentRequired`, etc.) + `const`→`enum`.
5. **set `info.version`** (Wise ships `version: ''`).

Tradeoff: this loses validation fidelity (fine for a runtime proxy). A **3.1-native,
lenient** tool (e.g. Stoplight **Prism** in `proxy` mode) would skip most of step 1–5
but isn't MCP. SCA-gated ops (fund/convert) still need request signing regardless.

### Other no-code consumers of the same spec
`restish` (instant CLI), Prism (mock/validating proxy), Speakeasy/Fern/Stainless
(typed SDKs + MCP). All point at `reference/wise-openapi.yaml` (or the 3.0 variant).

> **When do you still want the Rust ConnectRPC below?** Only if you want to *re-expose*
> Wise as your own typed RPC surface (one contract → Rust + TS + GUI). To merely *use*
> Wise, section 0 is enough.

---

## 1. Architecture (two boundaries)

```
        ┌─────────────────────── boundary A (upstream) ───────────────────────┐
Wise OpenAPI (published)  ──►  generated Rust Wise client (models + native HTTP)
                                              │  (+ thin worker::Fetch transport for wasm)
                                              ▼
                                   your Rust ConnectRPC service impl
                                              ▲
proto/wise/v1/wise.proto  ──►  connect server stubs + TS client + service-OpenAPI(docs)
        └────────────────────── boundary B (downstream) ──────────────────────┘
                                              ▼
                                     web GUI / other clients
```

- **Boundary A** = your service calling Wise. **Generate** this from Wise's OpenAPI.
- **Boundary B** = clients calling your service. **Hand-write** the slim proto contract.

The Go SDK already in this repo is a separate, parallel, hand-written implementation;
the only artifact shared with the Rust service is the proto.

---

## 2. Endpoint / schema source of truth (local — never re-scrape)

| File | What |
|------|------|
| [`reference/wise-openapi.yaml`](reference/wise-openapi.yaml) | **Official** Wise OpenAPI 3.1 bundle, verbatim. 198 paths / 239 ops / 131 schemas, self-contained. |
| [`reference/wise-endpoints.txt`](reference/wise-endpoints.txt) | Flat greppable `METHOD path — summary` index. Browse first. |
| [`reference/README.md`](reference/README.md) | Provenance + refresh. |
| [`../API.md`](../API.md) | Older hand-curated Go-coverage matrix. Where it disagrees with the spec, **trust the spec** (recipients=`/v2/accounts`, cards=`/v4/spend/...`). |

Refresh everything: **`mise run wise:spec`** (curl official URL → regenerate index).

---

## 3. Codegen — how to leverage the published OpenAPI

The spec is genuinely codegen-grade (verified from the local file):
**OpenAPI 3.1.0 · all 239 ops have `operationId` · 131 schemas · 50 tags · prod+sandbox
servers · 4 security schemes (UserToken/PersonalToken/ClientCredentialsToken/BasicAuth)**.
The one hard part: **35 `oneOf` + 26 `discriminator`** (dynamic recipient/requirements
polymorphism).

### Boundary A — upstream Wise client: **GENERATE** ✅ (the big win)
239 ops + 131 typed schemas for free, and it tracks Wise's updates (soenneker
regenerates its .NET client daily from the same URL).

| Tool | 3.1 | oneOf/discriminator | Notes |
|------|-----|---------------------|-------|
| [`gpu-cli/openapi-to-rust`](https://github.com/gpu-cli/openapi-to-rust) | **native 3.1** | auto tagged-unions from `oneOf`+`const` | purpose-built for 3.1; structs + HTTP + SSE. **Try first.** |
| [`openapi-generator` (rust)](https://openapi-generator.tech/docs/generators/rust/) | partial/improving | basic; known discriminator bugs ([#13257](https://github.com/OpenAPITools/openapi-generator/issues/13257), [#19194](https://github.com/OpenAPITools/openapi-generator/issues/19194)) | mature, Java jar via npx; reqwest client |
| `progenitor` (Oxide) | 3.0-centric | limited | great ergonomics, weaker 3.1 |

- **Fallback** if a generator chokes on 3.1: downconvert 3.1→3.0 (redocly / apidevtools
  converter) first, or pass `--skip-validate-spec`.
- **wasm/Workers caveat:** generated clients are `reqwest`-based → fine for the
  `native` cargo feature. For the `worker` feature (`worker::Fetch`, wasm32), **reuse
  the generated models only** + a thin hand-written transport — the same dual-transport
  split google_maps already ships. So: *generate models (transport-agnostic) + native
  client; hand-adapt the worker transport.*
- The **polymorphism lives here**, in serde tagged/untagged enums — not forced through proto.

### Boundary B — your ConnectRPC contract: **HAND-WRITE proto, generate outward**
The ecosystem direction is **proto → OpenAPI** (well supported: `protoc-gen-connect-openapi`),
**not OpenAPI → proto** (lossy, niche, breaks on `oneOf`/`discriminator`). So:

1. Hand-write the slim [`../proto/wise/v1/wise.proto`](../proto/wise/v1/wise.proto)
   (curated public contract; messages carry only what a client/GUI needs).
2. `connectrpc-build` (build.rs) → Rust server stubs; `protoc-gen-es` → TS client.
3. *(optional)* `protoc-gen-connect-openapi` → emit an OpenAPI of **your** service for docs.

> **Do NOT generate the proto from the Wise OpenAPI.** Tried-and-rejected: the direction
> is unsupported and the 35 oneOf / 26 discriminator types make it lossy. Hand-write.

### Proposed mise tasks (to wire next)
- `gen:client` — run the chosen OpenAPI→Rust generator over `reference/wise-openapi.yaml`
- `proto:gen` — `buf generate` (Rust stubs + TS client)
- `wise:spec` — already exists (refresh the upstream spec + index)

---

## 4. Rust implementation plan (mirrors `google_maps/crates/connectrpc`)

1. Crate `crates/connectrpc/` — `proto/`, `build.rs`, `src/server/mod.rs` (central trait
   dispatch) + one module per domain, `src/wise_gen/` (generated client), thin
   `src/transport_worker.rs` for wasm.
2. Two runtimes via cargo features: `native` (reqwest + axum), `worker` default
   (`worker::Fetch` + `#[event(fetch)]`, `worker-build`/`wrangler`). Apply
   `.into_send().await` on every Worker future (workers-rs + connectrpc `Send`).
3. Auth: Bearer via a tower layer (reuse google_maps `TokenAuthLayer`). **SCA is the
   one hard part** — funding transfers / converting balances need a signed `X-Signature`
   (RSA) + `X-2FA-Approval`; everything else is a thin REST→RPC proxy.
4. Secrets: fnox keychain (same as this repo).

**Where it lives:** a dedicated Rust crate/repo (`crates/connectrpc` member or a
`wise-bank-connectrpc` repo), NOT in the Go module.

---

## 5. Sources & decision log (so we can always revisit)

Worked out 2026-06-09. Re-derive with the searches below if anything goes stale.

**Reference repos (local siblings):**
- `joeblew999/google_maps` — `crates/connectrpc`: the Rust ConnectRPC pattern
  (proto → connectrpc-build, native+worker cargo features, protoc-gen-es TS client, GUI).
- `joeblew999/cf-do-locator` — same pattern on CF Workers; `.into_send()` rule, fnox/KV.
- `soenneker/soenneker.wise.openapiclient` (+ `...runners.openapiclient`) — .NET client
  generated daily from Wise's OpenAPI. **This is where we found the official spec URL.**

**Official Wise OpenAPI URL (the key find):**
`https://docs.wise.com/_bundle/api-reference/index.yaml` — vendored to
`reference/wise-openapi.yaml`. (Wise's HTML docs truncate on fetch; use this instead.)

**Spec facts (measured from the local file):** OpenAPI 3.1.0; 198 paths / 239 ops; all
ops have operationId; 131 schemas; 50 tags; servers prod+sandbox; security
UserToken/PersonalToken/ClientCredentialsToken/BasicAuth; 35 oneOf, 26 discriminator,
2 allOf.

**Tooling findings:** OpenAPI→Rust is viable (openapi-to-rust 3.1-native;
openapi-generator mature but 3.1/discriminator rough edges). OpenAPI→proto is NOT a
supported direction (ecosystem does proto→OpenAPI) → hand-write the proto.

**Searches that produced this:**
- `progenitor openapi-generator rust client OpenAPI 3.1 oneOf discriminator`
- `generate protobuf from OpenAPI ... openapi to proto connect-go buf`
- `Wise transferwise OpenAPI spec yaml json` → Postman collection + apitracker + soenneker

**Decisions:**
1. Generate boundary A (upstream Wise client) from OpenAPI; keep the Go SDK as the
   hand-written reference. ✅
2. Hand-write boundary B (the ConnectRPC proto); generate outward (TS client, docs). ✅
3. Reuse generated *models* + hand-written worker transport for the wasm target. ✅
4. API.md is legacy; `reference/wise-openapi.yaml` is the source of truth. ✅
