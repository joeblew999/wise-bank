# wise-bank - Wise Banking Platform

Go client for the [Wise API](https://docs.wise.com/api-reference) with CLI, MCP server, and web GUI.

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
├── commands/         # Shared business logic (DRY)
│   └── commands.go
├── cmd/
│   ├── wise-cli/     # CLI tool
│   ├── wise-mcp/     # MCP server for Claude
│   └── wise-server/  # Web GUI with Via
├── mise.toml         # Tools (Go, nushell) + task definitions
├── API.md            # API coverage documentation
└── README.md         # Quick start
```

## Architecture

Three layers, fan-out to three frontends. Everything below `commands` is reusable;
everything above it is just formatting for a medium.

```
   frontends    wise-cli        wise-mcp         wise-server      (cmd/*)
   (thin)       terminal        Claude/MCP       web GUI + SSE
                    \               |               /
                     \              |              /
   shared logic  ── commands package (DRY) ──  returns plain result structs
   (no I/O fmt)         GetRates / GetProfiles / GetBalances / ...
                                    |
   core library  ────────── package wise ──────────  Client + per-domain Services
                     client.go (one generic Request) + profiles/quotes/rates/...
                                    |
                              Wise REST API
```

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
structs (`RateResult`, `ProfileResult`, `BalanceResult`, ...). This is the DRY seam
that keeps the three frontends from duplicating API logic.

### Layer 3 — frontends (`cmd/*`), each just formats `commands` output
- **wise-cli** — prints results to the terminal
- **wise-mcp** — registers MCP tools (`wise_rates`, `wise_profiles`, ...) via
  `mark3labs/mcp-go`, serves over stdio for Claude
- **wise-server** — web dashboard (Via framework, SSE for live updates); the only
  frontend that supports the OAuth path

### Code generation
**None.** This is a hand-written SDK — no `go:generate`, no OpenAPI/Swagger codegen,
no `// DO NOT EDIT`. Adding a Wise endpoint = hand-write a method on the relevant
service (and a `commands` helper + frontend formatting if it should be user-facing).
Wise does publish an OpenAPI spec, so a generated client is possible, but the
deliberate choice here is hand-written for clean ergonomics over full coverage.

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

# run Wise as MCP
mise run mcp:openapi  # ▶ the WHOLE Wise API (239 ops) as MCP, off the local spec
mise run mcp:go       # hand-written Go MCP server (curated subset)

# CLI commands (need WISE_API_TOKEN)
mise run cli:rates        # Get exchange rates
mise run cli:profiles     # List profiles
mise run cli:balances     # Show balances
mise run cli:statements   # Transaction history
mise run cli:quote -- -from USD -to EUR -amount 100
mise run cli:rate-history -- -from EUR -to USD -days 7

# web GUI
mise run serve:web    # Start web dashboard (port 8080)

# spec + build/test
mise run spec:fetch   # refresh the official OpenAPI + endpoint index
mise run build:all    # Build all Go binaries into ./.bin
mise run test         # Run tests

# hidden (still runnable): spec:normalize, build:mcp, build:server, lint, clean,
#   mcp:oauth, serve:oauth, secrets:set-token, secrets:set-oauth, secrets:list, secrets:doctor
```

## CLI Help

```bash
./wise-cli -h                        # General help
./wise-cli -cmd help rate-history    # Help for specific command
```

## MCP Server Tools

- `wise_rates` - Get exchange rates between currency pairs
- `wise_profiles` - List all Wise profiles
- `wise_balances` - Show account balances
- `wise_statements` - Get transaction history
- `wise_quote` - Get currency conversion quotes
- `wise_rate_history` - Get historical exchange rates

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
