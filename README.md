# wise-bank

A toolkit for the [Wise (TransferWise) API](https://docs.wise.com/api-reference).

The whole Wise API is **driven from Wise's official OpenAPI spec** (vendored locally),
so you get the *entire* surface — as a CLI, as an MCP server, or as a web dashboard —
plus a hand-written Go SDK for embedding. All tooling and secrets run through
[mise](https://mise.jdx.dev) + [fnox](https://github.com/jdx/fnox).

## Quick start

```bash
mise install              # pinned tools: go, nushell, node, restish
mise run secrets:set      # store your Wise API token in the OS keychain (hidden prompt)
mise run secrets:status   # see which credentials are set (prod / sandbox / oauth)

mise tasks                # list everything
```

## Use the whole Wise API (from the spec)

| Interface | Command |
|-----------|---------|
| **CLI** (every endpoint, ~205 commands) | `mise run api:setup` then `mise run api -- rate-get --source=USD --target=EUR` |
| ↳ list all commands | `mise run api -- --help` |
| ↳ against the sandbox | `mise run api:sandbox -- rate-get --source=USD --target=EUR` |
| **MCP server** (for Claude / agents) | `mise run mcp:openapi` → 239 tools on `:9090/wise/mcp` |
| **Web dashboard** | `mise run serve:web` → `http://localhost:8080` |

These need no hand-written code — they read `docs/reference/wise-openapi.yaml`
(the official spec). Refresh it anytime with `mise run spec:fetch`.

## Or use the Go SDK / curated CLI

A hand-written Go library (`package wise`) with a small convenience CLI and MCP server:

```bash
mise run build:all    # build wise-cli, wise-mcp, wise-server into ./.bin
mise run test
mise run cli:rates    # curated subset: rates / profiles / balances / statements / quote / rate-history
mise run mcp:go       # hand-written Go MCP server
```

As a library:

```go
package main

import (
    "context"
    "fmt"
    "os"

    wise "github.com/joeblew999/wise-bank"
)

func main() {
    client := wise.NewClient(os.Getenv("WISE_API_TOKEN")) // or wise.WithSandbox()
    ctx := context.Background()

    profiles, _ := client.Profiles.List(ctx)
    rate, _ := client.ExchangeRates.Get(ctx, wise.USD, wise.EUR)
    fmt.Printf("profiles=%+v  USD/EUR=%f\n", profiles, rate.Rate)
}
```

`go get github.com/joeblew999/wise-bank`

## Secrets (fnox + OS keychain)

Credentials live in the OS keychain, never in the repo or a `.env`. Tasks that hit
the API run under `fnox exec`, which injects them at run time.

```bash
mise run secrets:open         # open the Wise token pages (prod + sandbox)
mise run secrets:set          # WISE_API_TOKEN        (production — required)
mise run secrets:set-sandbox  # WISE_SANDBOX_API_TOKEN (write/SCA testing)
mise run secrets:set-oauth    # WISE_CLIENT_ID/SECRET  (optional: partner/multi-user)
mise run secrets:status       # ✓/✗ for all of the above (values never shown)
```

Auth is **either/or**: the API token alone covers everything for your own account;
OAuth is only for partner / multi-user (PSD2) flows.

Create a production token at
<https://wise.com/your-account/integrations-and-tools/api-tokens>.

**Sandbox is a separate environment** — you can't use your real Wise account. Register
a sandbox-only account at <https://sandbox.transferwise.tech/register>, finish
onboarding, then create a token under that account's
*Integrations and tools → API tokens*.

## How it works

- `docs/reference/wise-openapi.yaml` — Wise's **official OpenAPI 3.1 bundle**, vendored
  verbatim. `docs/reference/wise-endpoints.txt` is a greppable index. `mise run spec:fetch`
  refreshes both.
- `mise run spec:normalize` produces a 3.0 variant that strict tools (restish, Go MCP
  proxy) accept. See [`docs/connectrpc.md`](docs/connectrpc.md) for the full design,
  codegen notes, and decision log.
- Tasks are namespaced (`api:` `mcp:` `serve:` `cli:` `secrets:` `spec:` `build:`);
  `mise tasks` shows the dev-facing set, plumbing is hidden. See [CLAUDE.md](CLAUDE.md).

## Links

- [Wise API reference](https://docs.wise.com/api-reference)
- [Auth & security](https://docs.wise.com/guides/developer/auth-and-security)
