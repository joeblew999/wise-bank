package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	wise "github.com/joeblew999/wise-bank"
	"github.com/joeblew999/wise-bank/commands"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	client *wise.Client
)

func main() {
	token := os.Getenv("WISE_API_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "WISE_API_TOKEN environment variable required")
		os.Exit(1)
	}

	var opts []wise.ClientOption
	if os.Getenv("WISE_SANDBOX") == "true" {
		opts = append(opts, wise.WithSandbox())
	}
	client = wise.NewClient(token, opts...)

	s := server.NewMCPServer(
		"wise-api",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	registerTools(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func getStringArg(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func getFloatArg(args map[string]any, key string, defaultVal float64) float64 {
	if v, ok := args[key].(float64); ok {
		return v
	}
	return defaultVal
}

func registerTools(s *server.MCPServer) {
	// Rates tool
	s.AddTool(
		mcp.NewTool("wise_rates",
			mcp.WithDescription("Get exchange rates between currency pairs"),
			mcp.WithString("from", mcp.Description("Source currency code (e.g., USD, EUR, GBP)"), mcp.Required()),
			mcp.WithString("to", mcp.Description("Target currency code (e.g., USD, EUR, GBP)"), mcp.Required()),
		),
		handleRates,
	)

	// Profiles tool
	s.AddTool(
		mcp.NewTool("wise_profiles",
			mcp.WithDescription("List all Wise profiles for the authenticated user"),
		),
		handleProfiles,
	)

	// Balances tool
	s.AddTool(
		mcp.NewTool("wise_balances",
			mcp.WithDescription("Show account balances across all profiles and currencies"),
		),
		handleBalances,
	)

	// Statements tool
	s.AddTool(
		mcp.NewTool("wise_statements",
			mcp.WithDescription("Get transaction history for the last N days"),
			mcp.WithNumber("days", mcp.Description("Number of days of history (default 30)")),
		),
		handleStatements,
	)

	// Quote tool
	s.AddTool(
		mcp.NewTool("wise_quote",
			mcp.WithDescription("Get a quote for currency conversion"),
			mcp.WithString("from", mcp.Description("Source currency code (e.g., USD, EUR)"), mcp.Required()),
			mcp.WithString("to", mcp.Description("Target currency code (e.g., USD, EUR)"), mcp.Required()),
			mcp.WithNumber("amount", mcp.Description("Amount to convert in source currency"), mcp.Required()),
		),
		handleQuote,
	)

	// Rate History tool
	s.AddTool(
		mcp.NewTool("wise_rate_history",
			mcp.WithDescription("Get historical exchange rates over a period"),
			mcp.WithString("from", mcp.Description("Source currency code (e.g., USD, EUR)"), mcp.Required()),
			mcp.WithString("to", mcp.Description("Target currency code (e.g., USD, EUR)"), mcp.Required()),
			mcp.WithNumber("days", mcp.Description("Number of days of history (default 7)")),
			mcp.WithString("group", mcp.Description("Grouping interval: day, hour, minute (default day)")),
		),
		handleHistory,
	)
}

func handleRates(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments.(map[string]any)
	from := getStringArg(args, "from")
	to := getStringArg(args, "to")

	result := commands.GetRate(ctx, client, from, to)
	if result.Error != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", result.Error)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("%s/%s: %.6f", result.From, result.To, result.Rate)), nil
}

func handleProfiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	profiles, err := commands.GetProfiles(ctx, client)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	if len(profiles) == 0 {
		return mcp.NewToolResultText("No profiles found"), nil
	}

	var lines []string
	for _, p := range profiles {
		lines = append(lines, fmt.Sprintf("ID: %d, Type: %s", p.ID, p.Type))
	}
	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

func handleBalances(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	results, err := commands.GetBalances(ctx, client)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No profiles found"), nil
	}

	var lines []string
	for _, r := range results {
		if r.Error != nil {
			lines = append(lines, fmt.Sprintf("Profile %d: error - %v", r.ProfileID, r.Error))
			continue
		}
		lines = append(lines, fmt.Sprintf("Profile %d (%s):", r.ProfileID, r.ProfileType))
		for _, b := range r.Balances {
			lines = append(lines, fmt.Sprintf("  %s: %.2f", b.Currency, b.Amount))
		}
	}
	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

func handleStatements(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments.(map[string]any)
	days := int(getFloatArg(args, "days", 30))

	results, err := commands.GetStatements(ctx, client, days)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Statements (last %d days):", days))

	for _, r := range results {
		if r.Error != nil {
			lines = append(lines, fmt.Sprintf("%s: error - %v", r.Currency, r.Error))
			continue
		}
		lines = append(lines, fmt.Sprintf("\n%s (Balance ID: %d):", r.Currency, r.BalanceID))
		if len(r.Transactions) == 0 {
			lines = append(lines, "  No transactions")
			continue
		}
		for _, t := range r.Transactions {
			lines = append(lines, fmt.Sprintf("  %s | %s | %.2f %s", t.Date, t.Type, t.Amount, t.Currency))
		}
	}
	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

func handleQuote(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments.(map[string]any)
	from := getStringArg(args, "from")
	to := getStringArg(args, "to")
	amount := getFloatArg(args, "amount", 0)

	if amount <= 0 {
		return mcp.NewToolResultError("Amount must be greater than 0"), nil
	}

	result := commands.GetQuote(ctx, client, from, to, amount)
	if result.Error != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", result.Error)), nil
	}

	output := map[string]interface{}{
		"from":         result.From,
		"to":           result.To,
		"sourceAmount": result.SourceAmount,
		"targetAmount": result.TargetAmount,
		"rate":         result.Rate,
		"quoteId":      result.QuoteID,
		"expires":      result.Expires,
	}

	jsonBytes, _ := json.MarshalIndent(output, "", "  ")
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

func handleHistory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments.(map[string]any)
	from := getStringArg(args, "from")
	to := getStringArg(args, "to")
	days := int(getFloatArg(args, "days", 7))
	group := getStringArg(args, "group")
	if group == "" {
		group = "day"
	}

	result := commands.GetRateHistory(ctx, client, from, to, days, group)
	if result.Error != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", result.Error)), nil
	}

	output := map[string]interface{}{
		"from":       result.From,
		"to":         result.To,
		"dataPoints": len(result.DataPoints),
		"first":      result.First,
		"last":       result.Last,
		"min":        result.Min,
		"max":        result.Max,
		"history":    result.DataPoints,
	}

	jsonBytes, _ := json.MarshalIndent(output, "", "  ")
	return mcp.NewToolResultText(string(jsonBytes)), nil
}
