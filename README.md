# Wise API Go Client

Go client for the [Wise API](https://docs.wise.com/api-reference), with a CLI, an MCP server, and a web GUI.

## Requirements

- [mise](https://mise.jdx.dev) — manages tools (Go, nushell) and runs all tasks

```bash
mise install          # install pinned tools (Go, nushell, node)
mise tasks            # list available tasks

# secrets — stored in the OS keychain via fnox (see fnox.toml)
mise run secrets:open # open the Wise pages to create your API token / OAuth app
mise run secrets:set  # store WISE_API_TOKEN in the keychain (hidden prompt)

# run the WHOLE Wise API (239 ops) as an MCP server — straight from the spec
mise run mcp:openapi

# or use the Go SDK directly
mise run build:all    # build binaries into ./.bin
mise run test
mise run serve:web    # web dashboard on :8080 (uses the keychain token)
mise run cli:rates    # any API command, e.g. exchange rates
```

Secrets never live in this repo — `fnox` injects them from the keychain at run
time. For OAuth (partner / multi-user) use `mise run secrets:set-oauth` and the
`:oauth` task variants. See [CLAUDE.md](CLAUDE.md) for the full task list.

## Installation (as a library)

```bash
go get github.com/joeblew999/wise-bank
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "os"

    wise "github.com/joeblew999/wise-bank"
)

func main() {
    // Create client (uses production by default)
    client := wise.NewClient(os.Getenv("WISE_API_TOKEN"))

    // Or use sandbox
    // client := wise.NewClient(os.Getenv("WISE_API_TOKEN"), wise.WithSandbox())

    ctx := context.Background()

    // Get profiles
    profiles, _ := client.Profiles.List(ctx)
    fmt.Printf("Profiles: %+v\n", profiles)

    // Get exchange rate
    rate, _ := client.ExchangeRates.Get(ctx, wise.USD, wise.EUR)
    fmt.Printf("USD/EUR: %f\n", rate.Rate)

    // Get balances
    balances, _ := client.Balances.List(ctx, profiles[0].ID, nil)
    fmt.Printf("Balances: %+v\n", balances)
}
```

## Services

- **Profiles** - List, get, create personal/business profiles
- **Quotes** - Create and manage transfer quotes
- **Recipients** - Manage recipient accounts
- **Transfers** - Create, list, cancel transfers
- **ExchangeRates** - Get live and historical exchange rates
- **Balances** - Multi-currency balance management

## Environment Variables

```bash
WISE_API_TOKEN=your-api-token
```

## API Reference

https://docs.wise.com/api-reference
