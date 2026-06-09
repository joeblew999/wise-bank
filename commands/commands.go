// Package commands provides shared business logic for Wise API operations.
// Both CLI and MCP use this package to avoid code duplication.
package commands

import (
	"context"
	"fmt"
	"time"

	wise "github.com/joeblew999/wise-bank"
)

// RateResult holds an exchange rate result.
type RateResult struct {
	From  string
	To    string
	Rate  float64
	Error error
}

// ProfileResult holds a profile result.
type ProfileResult struct {
	ID   int64
	Type string
}

// BalanceResult holds balance information for a profile.
type BalanceResult struct {
	ProfileID   int64
	ProfileType string
	Balances    []CurrencyBalance
	Error       error
}

// CurrencyBalance holds a single currency balance.
type CurrencyBalance struct {
	Currency string
	Amount   float64
}

// StatementResult holds statement information.
type StatementResult struct {
	Currency    string
	BalanceID   int64
	Transactions []Transaction
	Error       error
}

// Transaction holds a single transaction.
type Transaction struct {
	Date     string
	Type     string
	Amount   float64
	Currency string
}

// QuoteResult holds a quote result.
type QuoteResult struct {
	From         string
	To           string
	SourceAmount float64
	TargetAmount float64
	Rate         float64
	QuoteID      string
	Expires      string
	Error        error
}

// HistoryResult holds rate history information.
type HistoryResult struct {
	From       string
	To         string
	DataPoints []HistoryPoint
	Min        float64
	Max        float64
	First      float64
	Last       float64
	Error      error
}

// HistoryPoint holds a single historical rate point.
type HistoryPoint struct {
	Time string
	Rate float64
}

// GetRates fetches exchange rates for common currency pairs.
func GetRates(ctx context.Context, client *wise.Client) []RateResult {
	pairs := [][2]wise.Currency{
		{wise.USD, wise.EUR},
		{wise.GBP, wise.USD},
		{wise.EUR, wise.GBP},
		{wise.USD, wise.JPY},
	}

	results := make([]RateResult, 0, len(pairs))
	for _, pair := range pairs {
		result := RateResult{From: string(pair[0]), To: string(pair[1])}
		rate, err := client.ExchangeRates.Get(ctx, pair[0], pair[1])
		if err != nil {
			result.Error = err
		} else {
			result.Rate = rate.Rate
		}
		results = append(results, result)
	}
	return results
}

// GetRate fetches a single exchange rate.
func GetRate(ctx context.Context, client *wise.Client, from, to string) RateResult {
	result := RateResult{From: from, To: to}
	rate, err := client.ExchangeRates.Get(ctx, wise.Currency(from), wise.Currency(to))
	if err != nil {
		result.Error = err
	} else {
		result.Rate = rate.Rate
	}
	return result
}

// GetProfiles fetches all profiles.
func GetProfiles(ctx context.Context, client *wise.Client) ([]ProfileResult, error) {
	profiles, err := client.Profiles.List(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]ProfileResult, 0, len(profiles))
	for _, p := range profiles {
		results = append(results, ProfileResult{ID: p.ID, Type: string(p.Type)})
	}
	return results, nil
}

// GetBalances fetches balances for all profiles.
func GetBalances(ctx context.Context, client *wise.Client) ([]BalanceResult, error) {
	profiles, err := client.Profiles.List(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]BalanceResult, 0, len(profiles))
	for _, p := range profiles {
		result := BalanceResult{ProfileID: p.ID, ProfileType: string(p.Type)}
		balances, err := client.Balances.List(ctx, p.ID, nil)
		if err != nil {
			result.Error = err
		} else {
			for _, b := range balances {
				result.Balances = append(result.Balances, CurrencyBalance{
					Currency: string(b.Currency),
					Amount:   b.Amount.Value,
				})
			}
		}
		results = append(results, result)
	}
	return results, nil
}

// GetStatements fetches statements for all profiles.
func GetStatements(ctx context.Context, client *wise.Client, days int) ([]StatementResult, error) {
	if days <= 0 {
		days = 30
	}

	profiles, err := client.Profiles.List(ctx)
	if err != nil {
		return nil, err
	}

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -days)
	startStr := start.Format(time.RFC3339)
	endStr := end.Format(time.RFC3339)

	var results []StatementResult
	for _, p := range profiles {
		balances, err := client.Balances.List(ctx, p.ID, nil)
		if err != nil {
			results = append(results, StatementResult{Error: fmt.Errorf("profile %d: %w", p.ID, err)})
			continue
		}

		for _, b := range balances {
			if b.Amount.Value == 0 {
				continue
			}
			result := StatementResult{Currency: string(b.Currency), BalanceID: b.ID}
			statements, err := client.Balances.GetStatement(ctx, p.ID, b.ID, b.Currency, startStr, endStr)
			if err != nil {
				result.Error = err
			} else {
				for _, s := range statements {
					result.Transactions = append(result.Transactions, Transaction{
						Date:     s.Date.Format("2006-01-02"),
						Type:     s.Type,
						Amount:   s.Amount.Value,
						Currency: string(s.Amount.Currency),
					})
				}
			}
			results = append(results, result)
		}
	}
	return results, nil
}

// GetQuote creates a quote for currency conversion.
func GetQuote(ctx context.Context, client *wise.Client, from, to string, amount float64) QuoteResult {
	result := QuoteResult{From: from, To: to, SourceAmount: amount}

	profiles, err := client.Profiles.List(ctx)
	if err != nil {
		result.Error = err
		return result
	}

	if len(profiles) == 0 {
		result.Error = fmt.Errorf("no profiles found")
		return result
	}

	req := &wise.CreateQuoteRequest{
		SourceCurrency: wise.Currency(from),
		TargetCurrency: wise.Currency(to),
		SourceAmount:   &amount,
		Profile:        profiles[0].ID,
	}

	quote, err := client.Quotes.CreateV2(ctx, req)
	if err != nil {
		result.Error = err
		return result
	}

	result.TargetAmount = quote.TargetAmount
	if result.TargetAmount == 0 && len(quote.PaymentOptions) > 0 {
		result.TargetAmount = quote.PaymentOptions[0].TargetAmount
	}
	result.Rate = quote.Rate
	result.QuoteID = quote.ID
	result.Expires = quote.RateExpirationTime.Format("2006-01-02 15:04:05")

	return result
}

// GetRateHistory fetches historical exchange rates over a period.
// group can be "day", "hour", or "minute"
func GetRateHistory(ctx context.Context, client *wise.Client, from, to string, days int, group string) HistoryResult {
	result := HistoryResult{From: from, To: to}

	if days <= 0 {
		days = 7
	}
	if group == "" {
		group = "day"
	}

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -days)

	params := &wise.HistoryParams{
		Source: wise.Currency(from),
		Target: wise.Currency(to),
		From:   start.Format(time.RFC3339),
		To:     end.Format(time.RFC3339),
		Group:  group,
	}

	rates, err := client.ExchangeRates.GetHistory(ctx, params)
	if err != nil {
		result.Error = err
		return result
	}

	if len(rates) == 0 {
		result.Error = fmt.Errorf("no historical data found")
		return result
	}

	// Calculate stats
	result.First = rates[0].Rate
	result.Last = rates[len(rates)-1].Rate
	result.Min = rates[0].Rate
	result.Max = rates[0].Rate

	for _, r := range rates {
		result.DataPoints = append(result.DataPoints, HistoryPoint{
			Time: r.Time.Format("2006-01-02 15:04"),
			Rate: r.Rate,
		})
		if r.Rate < result.Min {
			result.Min = r.Rate
		}
		if r.Rate > result.Max {
			result.Max = r.Rate
		}
	}

	return result
}
