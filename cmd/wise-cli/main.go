package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	wise "github.com/joeblew999/wise-bank"
	"github.com/joeblew999/wise-bank/commands"
)

var cmdHelp = map[string]struct {
	desc   string
	usage  string
	flags  []string
}{
	"rates": {
		desc:  "Get exchange rates for common currency pairs",
		usage: "wise-cli -cmd rates",
		flags: []string{},
	},
	"profiles": {
		desc:  "List all Wise profiles for the authenticated user",
		usage: "wise-cli -cmd profiles",
		flags: []string{},
	},
	"balances": {
		desc:  "Show account balances across all profiles and currencies",
		usage: "wise-cli -cmd balances",
		flags: []string{},
	},
	"statements": {
		desc:  "Get transaction history for the last N days",
		usage: "wise-cli -cmd statements [-days 30]",
		flags: []string{"days"},
	},
	"quote": {
		desc:  "Get a quote for currency conversion",
		usage: "wise-cli -cmd quote -from USD -to EUR -amount 100",
		flags: []string{"from", "to", "amount"},
	},
	"rate-history": {
		desc:  "Get historical exchange rates over a period",
		usage: "wise-cli -cmd rate-history -from EUR -to USD [-days 7] [-group day]",
		flags: []string{"from", "to", "days", "group"},
	},
	"help": {
		desc:  "Show help for a specific command",
		usage: "wise-cli -cmd help [command]",
		flags: []string{},
	},
}

func printUsage() {
	fmt.Println("Wise CLI - Command line interface for Wise API")
	fmt.Println()
	fmt.Println("Usage: wise-cli -cmd <command> [flags]")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  WISE_API_TOKEN    Required. Your Wise API token")
	fmt.Println()
	fmt.Println("Commands:")
	for name, help := range cmdHelp {
		fmt.Printf("  %-14s  %s\n", name, help.desc)
	}
	fmt.Println()
	fmt.Println("Global Flags:")
	fmt.Println("  -sandbox    Use sandbox environment")
	fmt.Println()
	fmt.Println("Use 'wise-cli -cmd help <command>' for more information about a command.")
}

func printCmdHelp(cmdName string) {
	help, ok := cmdHelp[cmdName]
	if !ok {
		fmt.Printf("Unknown command: %s\n", cmdName)
		fmt.Println()
		printUsage()
		os.Exit(1)
	}

	fmt.Printf("Command: %s\n", cmdName)
	fmt.Printf("  %s\n", help.desc)
	fmt.Println()
	fmt.Printf("Usage:\n  %s\n", help.usage)

	if len(help.flags) > 0 {
		fmt.Println()
		fmt.Println("Flags:")
		flagDescs := map[string]string{
			"from":   "Source currency code (e.g., USD, EUR, GBP)",
			"to":     "Target currency code (e.g., USD, EUR, GBP)",
			"amount": "Amount to convert in source currency",
			"days":   "Number of days (default varies by command)",
			"group":  "Grouping interval: day, hour, minute (default: day)",
		}
		for _, f := range help.flags {
			fmt.Printf("  -%-10s  %s\n", f, flagDescs[f])
		}
	}
}

func main() {
	cmd := flag.String("cmd", "rates", "Command to run")
	from := flag.String("from", "USD", "Source currency")
	to := flag.String("to", "EUR", "Target currency")
	amount := flag.Float64("amount", 100, "Amount for quote")
	days := flag.Int("days", 7, "Days of history")
	group := flag.String("group", "day", "History grouping: day, hour, minute")
	sandbox := flag.Bool("sandbox", false, "Use sandbox environment")

	flag.Usage = printUsage
	flag.Parse()

	// Handle help command
	if *cmd == "help" {
		args := flag.Args()
		if len(args) > 0 {
			printCmdHelp(args[0])
		} else {
			printUsage()
		}
		return
	}

	token := os.Getenv("WISE_API_TOKEN")
	if token == "" {
		fmt.Println("Error: WISE_API_TOKEN environment variable required")
		fmt.Println()
		printUsage()
		os.Exit(1)
	}

	var opts []wise.ClientOption
	if *sandbox {
		opts = append(opts, wise.WithSandbox())
	}
	client := wise.NewClient(token, opts...)
	ctx := context.Background()

	switch *cmd {
	case "rates":
		printRates(ctx, client)
	case "profiles":
		printProfiles(ctx, client)
	case "balances":
		printBalances(ctx, client)
	case "statements":
		printStatements(ctx, client, *days)
	case "quote":
		printQuote(ctx, client, *from, *to, *amount)
	case "rate-history":
		printHistory(ctx, client, *from, *to, *days, *group)
	default:
		fmt.Printf("Unknown command: %s\n", *cmd)
		fmt.Println()
		printUsage()
		os.Exit(1)
	}
}

func printRates(ctx context.Context, client *wise.Client) {
	results := commands.GetRates(ctx, client)
	fmt.Println("Exchange Rates:")
	fmt.Println("---------------")
	for _, r := range results {
		if r.Error != nil {
			fmt.Printf("%s/%s: error - %v\n", r.From, r.To, r.Error)
		} else {
			fmt.Printf("%s/%s: %.6f\n", r.From, r.To, r.Rate)
		}
	}
}

func printProfiles(ctx context.Context, client *wise.Client) {
	profiles, err := commands.GetProfiles(ctx, client)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Profiles:")
	fmt.Println("---------")
	if len(profiles) == 0 {
		fmt.Println("No profiles found")
		return
	}
	for _, p := range profiles {
		fmt.Printf("ID: %d, Type: %s\n", p.ID, p.Type)
	}
}

func printBalances(ctx context.Context, client *wise.Client) {
	results, err := commands.GetBalances(ctx, client)
	if err != nil {
		fmt.Printf("Error getting profiles: %v\n", err)
		return
	}

	if len(results) == 0 {
		fmt.Println("No profiles found")
		return
	}

	fmt.Println("Balances:")
	fmt.Println("---------")
	for _, r := range results {
		if r.Error != nil {
			fmt.Printf("Profile %d: error - %v\n", r.ProfileID, r.Error)
			continue
		}
		fmt.Printf("Profile %d (%s):\n", r.ProfileID, r.ProfileType)
		for _, b := range r.Balances {
			fmt.Printf("  %s: %.2f\n", b.Currency, b.Amount)
		}
	}
}

func printStatements(ctx context.Context, client *wise.Client, days int) {
	if days <= 0 {
		days = 30
	}
	results, err := commands.GetStatements(ctx, client, days)
	if err != nil {
		fmt.Printf("Error getting profiles: %v\n", err)
		return
	}

	fmt.Printf("Statements (last %d days):\n", days)
	fmt.Println("--------------------------")

	for _, r := range results {
		if r.Error != nil {
			fmt.Printf("%s: error - %v\n", r.Currency, r.Error)
			continue
		}
		fmt.Printf("\n%s (Balance ID: %d):\n", r.Currency, r.BalanceID)
		if len(r.Transactions) == 0 {
			fmt.Println("  No transactions")
			continue
		}
		for _, t := range r.Transactions {
			fmt.Printf("  %s | %s | %.2f %s\n", t.Date, t.Type, t.Amount, t.Currency)
		}
	}
}

func printQuote(ctx context.Context, client *wise.Client, from, to string, amount float64) {
	result := commands.GetQuote(ctx, client, from, to, amount)
	if result.Error != nil {
		fmt.Printf("Error: %v\n", result.Error)
		return
	}

	fmt.Println("Quote:")
	fmt.Println("------")
	fmt.Printf("  %s %.2f → %s %.2f\n", result.From, result.SourceAmount, result.To, result.TargetAmount)
	fmt.Printf("  Rate: %.6f\n", result.Rate)
	fmt.Printf("  Quote ID: %s\n", result.QuoteID)
	fmt.Printf("  Expires: %s\n", result.Expires)
}

func printHistory(ctx context.Context, client *wise.Client, from, to string, days int, group string) {
	result := commands.GetRateHistory(ctx, client, from, to, days, group)
	if result.Error != nil {
		fmt.Printf("Error: %v\n", result.Error)
		return
	}

	fmt.Printf("Rate History: %s/%s (last %d days)\n", result.From, result.To, days)
	fmt.Println("----------------------------------")
	fmt.Printf("  Data points: %d\n", len(result.DataPoints))
	fmt.Printf("  First: %.6f\n", result.First)
	fmt.Printf("  Last:  %.6f\n", result.Last)
	fmt.Printf("  Min:   %.6f\n", result.Min)
	fmt.Printf("  Max:   %.6f\n", result.Max)

	if len(result.DataPoints) > 0 {
		fmt.Println("\nRecent rates:")
		// Show last 10 points
		start := len(result.DataPoints) - 10
		if start < 0 {
			start = 0
		}
		for _, p := range result.DataPoints[start:] {
			fmt.Printf("  %s: %.6f\n", p.Time, p.Rate)
		}
	}
}
