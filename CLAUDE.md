# wise-bank - Wise Banking Platform

Toolkit for the [Wise API](https://docs.wise.com/api-reference): the whole API as a
spec-driven CLI (`api:*`, restish) and MCP server (`mcp:openapi`), plus a hand-written
Go library that powers a web GUI.

## Tooling

This project uses [mise](https://mise.jdx.dev) for both tool management (Go, nushell) and task running — `mise run <task>` works identically locally and in CI. File operations in tasks use nushell for OS-neutrality. (The old xplat/Taskfile/process-compose setup has been removed.)

## Important

**Keep mise.toml in sync with the API** - When adding new API endpoints or commands, always add a corresponding `mise` task.

## Structure

```
wise-bank/
├── client.go         # HTTP client with services
├── oauth.go          # OAuth 2.0 authentication
├── errors.go         # API error types
├── types.go          # Common types (Currency, Money, Timestamp)
├── profiles.go       # Profiles API
├── quotes.go         # Quotes API
├── recipients.go     # Recipients API
├── transfers.go      # Transfers API
├── rates.go          # Exchange rates API
├── balances.go       # Balances API
├── commands/         # Shared business logic (DRY) — used by the web server
│   └── commands.go
├── cmd/
│   └── wise-server/  # Web GUI with Via (the one Go frontend kept)
├── docs/reference/   # vendored official OpenAPI spec + endpoint index
├── scripts/          # nushell/node helpers (spec normalize, smoke, urls, ...)
├── openapi-mcp.yaml  # config for the whole-API MCP server
├── mise.toml         # Tools + task definitions
├── API.md            # Go SDK coverage matrix
└── README.md         # Quick start
```

## Architecture

Two ways to reach the whole Wise API, both off the vendored OpenAPI spec, plus a
hand-written Go library that powers the web GUI.

```
   WHOLE API (spec-driven, no hand-written code):
     restish  → `mise run api`          (CLI, ~205 commands)
     merzzzl  → `mise run mcp:openapi`  (MCP, 239 tools)
        both read docs/reference/wise-openapi.yaml

   Go library (hand-written) → powers:
     cmd/wise-server → `mise run serve:web`  (web GUI, Via + SSE)
        wise-server → commands/ (DRY) → package wise → Wise REST API
```

Note: the hand-written Go **CLI and MCP server were removed** — the spec-driven
`api:*` and `mcp:openapi` cover the entire API, so they were redundant. The Go
library + `commands/` are kept because the web GUI uses them.

### Layer 1 — core library (root package `wise`)
- `client.go` — a single generic `Request(ctx, method, path, query, body, result)`:
  marshals JSON, sets `Authorization: Bearer <token>`, executes, and either
  unmarshals the result or decodes an `APIError` on HTTP >= 400. `Get/Post/Put/Delete`
  are thin wrappers. Configured with option-functions (`WithSandbox`, `WithTimeout`,
  `WithHTTPClient`, `WithBaseURL`).
- **Service-per-domain** (go-github style): `Client` holds `Profiles`, `Quotes`,
  `Recipients`, `Transfers`, `ExchangeRates`, `Balances` — each a struct wrapping
  `*Client` with typed methods (e.g. `ExchangeRatesService.List(ctx, params)`).
  One domain per file (`rates.go`, `profiles.go`, ...).
- `types.go` — hand-modeled types: `Currency`, `Money`, status/profile enums, and a
  custom `Timestamp` whose `UnmarshalJSON` tries **7 date formats** because Wise
  returns inconsistent timestamps.

### Layer 2 — `commands/`
Shared business logic. Calls the library and returns flat, presentation-free result
structs (`RateResult`, `ProfileResult`, `BalanceResult`, ...). The web server formats
these for the browser.

### Layer 3 — frontend: `cmd/wise-server`
Web dashboard (Via framework, SSE for live updates); supports both the API-token and
OAuth paths. The former Go CLI (`wise-cli`) and Go MCP server (`wise-mcp`) were removed
in favour of the spec-driven `api:*` (restish) and `mcp:openapi`.

### Whole-API access is spec-driven (not hand-written)
`api:*` (restish) and `mcp:openapi` read the vendored OpenAPI and expose the entire
API with zero per-endpoint code. The Go library above is hand-written and intentionally
covers only what the web GUI needs — see `docs/design.md` for the design + decision
log, and `docs/reference/` for the spec.

## Authentication

Secrets are managed by **fnox** (see `fnox.toml`) and stored in the OS keychain —
never in the repo or a `.env`. Tasks that hit the API run under `fnox exec`, which
injects the credentials at run time, scoped per profile (`wise` for the token,
`oauth` for partner auth) with `--no-defaults` so they stay isolated from the
global fnox config.

```bash
mise run secrets:open       # open the Wise pages to create the credentials
mise run secrets:set        # store WISE_API_TOKEN in the keychain (token auth)
mise run secrets:set-oauth  # store WISE_CLIENT_ID + WISE_CLIENT_SECRET (OAuth)
mise run secrets:check      # verify what's configured
```

The raw environment variables below are what the binaries read — fnox provides
them. You can still `export` them manually for ad-hoc runs.

### API Token (Simple)
```bash
export WISE_API_TOKEN=your-token-here
```

### OAuth 2.0 (Multi-user / Partners)
```bash
export WISE_CLIENT_ID=your-client-id
export WISE_CLIENT_SECRET=your-client-secret
export WISE_REDIRECT_URL=http://localhost:8080/oauth/callback  # optional
```

OAuth flow:
1. User redirects to `wise.com/oauth/authorize`
2. User grants access
3. Wise redirects back with authorization code
4. Exchange code for access token
5. Token auto-refreshes (12 hour expiry)

## Wise API Endpoints

### Profiles
- `GET /v2/profiles` - List all profiles
- `GET /v2/profiles/{id}` - Get profile by ID

### Balances
- `GET /v4/profiles/{id}/balances` - List balances (requires `types=STANDARD`)
- `GET /v1/profiles/{id}/balance-statements/{balanceId}/statement.json` - Get statements

### Exchange Rates
- `GET /v1/rates` - Get rates (public, no auth needed)
- `GET /v1/rates?from=...&to=...&group=day` - Get rate history

### Quotes
- `POST /v2/quotes` - Create quote
- `GET /v2/quotes/{id}` - Get quote

### Recipients
- `POST /v1/accounts` - Create recipient
- `GET /v1/accounts` - List recipients

### Transfers
- `POST /v1/transfers` - Create transfer
- `GET /v1/transfers/{id}` - Get transfer

## Tasks

Tasks are namespaced; `mise tasks` shows the dev-facing set, plumbing is hidden
(`hide = true`, still runnable). `mise tasks --hidden` shows everything.

```bash
mise tasks            # dev-facing tasks

# the WHOLE Wise API, from the spec (no hand-written code)
mise run mcp:openapi               # as MCP — 239 tools
mise run api:setup                 # one-time: register the CLI with restish
mise run api -- --help             # list every command (~205)
mise run api -- rate-get --source=USD --target=EUR
mise run api:sandbox -- rate-get --source=USD --target=EUR   # against sandbox

# web GUI (Go)
mise run serve:web    # web dashboard on :8080

# secrets / sandbox
mise run secrets:urls         # every Wise URL (which page gives which thing)
mise run secrets:set          # store the production API token
mise run secrets:set-sandbox  # store the SANDBOX token
mise run secrets:status       # which secrets are set (prod/sandbox/oauth)

# spec + build/test/verify
mise run spec:fetch   # refresh the official OpenAPI + endpoint index
mise run build:all    # build the wise-server binary into ./.bin
mise run test         # Go tests
mise run smoke        # verify setup (offline); smoke:auth adds live token checks

# hidden (still runnable): spec:normalize, lint, clean, serve:oauth,
#   secrets:set-token, secrets:set-oauth, secrets:list, secrets:doctor
```

## Whole-API CLI / MCP (spec-driven)

```bash
mise run api -- --help                              # every Wise command (~205)
mise run api -- rate-get --source=USD --target=EUR  # any endpoint
mise run mcp:openapi                                 # all 239 ops as MCP tools
```

## Web GUI Features

- Profiles list
- Live account balances
- Exchange rate table
- Currency conversion quotes
- Transaction statements
- Rate history with charts
- OAuth login flow (when configured)
- Real-time updates via SSE (Via framework)

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `WISE_API_TOKEN` | Yes* | Personal API token |
| `WISE_CLIENT_ID` | Yes* | OAuth client ID |
| `WISE_CLIENT_SECRET` | Yes* | OAuth client secret |
| `WISE_REDIRECT_URL` | No | OAuth redirect (default: localhost) |
| `WISE_SANDBOX` | No | Set to "true" for sandbox |

*Either API token OR OAuth credentials required.

## Wise API Notes

- Access tokens expire after 12 hours
- Refresh tokens should be stored securely
- Some endpoints require OAuth (not personal tokens) in EU/UK due to PSD2
- Rate limits apply - check response headers
- Sandbox: `api.sandbox.transferwise.tech`
- Production: `api.wise.com`

## Links

- [Wise API Reference](https://docs.wise.com/api-reference)
- [Auth & Security Guide](https://docs.wise.com/guides/developer/auth-and-security)
- [OAuth User Tokens](https://docs.wise.com/api-reference/user-tokens)
