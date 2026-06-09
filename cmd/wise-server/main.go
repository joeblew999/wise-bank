package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"

	wise "github.com/joeblew999/wise-bank"
	"github.com/joeblew999/wise-bank/commands"

	"github.com/go-via/via"
	"github.com/go-via/via-plugin-picocss/picocss"
	. "github.com/go-via/via/h"
)

var (
	client      *wise.Client
	oauthClient *wise.OAuthClient
	tokenMgr    *wise.TokenManager
	mu          sync.RWMutex
	authMode    string // "token" or "oauth"
)

func main() {
	port := flag.String("port", "8080", "Server port")
	sandbox := flag.Bool("sandbox", false, "Use sandbox environment")
	flag.Parse()

	// Check for OAuth credentials first
	clientID := os.Getenv("WISE_CLIENT_ID")
	clientSecret := os.Getenv("WISE_CLIENT_SECRET")
	redirectURL := os.Getenv("WISE_REDIRECT_URL")

	if clientID != "" && clientSecret != "" {
		authMode = "oauth"
		if redirectURL == "" {
			redirectURL = fmt.Sprintf("http://localhost:%s/oauth/callback", *port)
		}
		oauthClient = wise.NewOAuthClient(wise.OAuthConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Sandbox:      *sandbox,
		})
		fmt.Println("OAuth mode enabled")
	} else {
		// Fall back to API token
		authMode = "token"
		token := os.Getenv("WISE_API_TOKEN")
		if token == "" {
			fmt.Println("Error: WISE_API_TOKEN or WISE_CLIENT_ID/WISE_CLIENT_SECRET required")
			os.Exit(1)
		}

		var opts []wise.ClientOption
		if *sandbox {
			opts = append(opts, wise.WithSandbox())
		}
		client = wise.NewClient(token, opts...)
		fmt.Println("API token mode enabled")
	}

	startServer(*port, *sandbox)
}

type AppData struct {
	Rates       []commands.RateResult
	Balances    []commands.BalanceResult
	Profiles    []commands.ProfileResult
	Statements  []commands.StatementResult
	RateHistory *commands.HistoryResult
	Quote       *commands.QuoteResult
	LoggedIn    bool
	AuthURL     string
	OAuthState  string
	AuthMode    string
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func getClient() *wise.Client {
	mu.RLock()
	defer mu.RUnlock()
	return client
}

func setClient(c *wise.Client) {
	mu.Lock()
	defer mu.Unlock()
	client = c
}

func startServer(port string, sandbox bool) {
	v := via.New()

	v.Config(via.Options{
		DocumentTitle: "Wise Account Dashboard",
		ServerAddress: ":" + port,
		Plugins: []via.Plugin{
			picocss.WithOptions(picocss.Options{
				Theme:         picocss.ThemeGreen,
				IncludeColors: true,
			}),
		},
	})

	// OAuth callback page
	if authMode == "oauth" {
		v.Page("/oauth/callback", func(c *via.Context) {
			c.View(func() H {
				return Main(Class("container"),
					Section(
						H1(Text("Processing OAuth...")),
						P(Text("Please wait while we complete authentication.")),
						Script(Text(`
							const params = new URLSearchParams(window.location.search);
							const code = params.get('code');
							const state = params.get('state');
							if (code) {
								fetch('/oauth/complete?code=' + code + '&state=' + state)
									.then(() => window.location.href = '/');
							}
						`)),
					),
				)
			})
		})
	}

	v.Page("/", func(c *via.Context) {
		ctx := context.Background()
		data := &AppData{
			AuthMode: authMode,
		}

		// Initialize state for OAuth
		if authMode == "oauth" {
			data.OAuthState = generateState()
			data.AuthURL = oauthClient.AuthURL(data.OAuthState)
			data.LoggedIn = getClient() != nil
		} else {
			data.LoggedIn = true // Always logged in with API token
		}

		fromCurrency := c.Signal("EUR")
		toCurrency := c.Signal("USD")
		amount := c.Signal(100.0)

		refreshRates := c.Action(func() {
			cl := getClient()
			if cl == nil {
				return
			}
			data.Rates = commands.GetRates(ctx, cl)
			c.Sync()
		})

		refreshBalances := c.Action(func() {
			cl := getClient()
			if cl == nil {
				return
			}
			balances, _ := commands.GetBalances(ctx, cl)
			data.Balances = balances
			c.Sync()
		})

		getQuote := c.Action(func() {
			cl := getClient()
			if cl == nil {
				return
			}
			from := fromCurrency.String()
			to := toCurrency.String()
			amt := amount.Float()
			result := commands.GetQuote(ctx, cl, from, to, amt)
			data.Quote = &result
			c.Sync()
		})

		refreshProfiles := c.Action(func() {
			cl := getClient()
			if cl == nil {
				return
			}
			profiles, _ := commands.GetProfiles(ctx, cl)
			data.Profiles = profiles
			c.Sync()
		})

		// Signals for statements
		statementDays := c.Signal(30)

		refreshStatements := c.Action(func() {
			cl := getClient()
			if cl == nil {
				return
			}
			days := int(statementDays.Float())
			statements, _ := commands.GetStatements(ctx, cl, days)
			data.Statements = statements
			c.Sync()
		})

		// Signals for rate history
		historyFrom := c.Signal("EUR")
		historyTo := c.Signal("USD")
		historyDays := c.Signal(7)

		getRateHistory := c.Action(func() {
			cl := getClient()
			if cl == nil {
				return
			}
			from := historyFrom.String()
			to := historyTo.String()
			days := int(historyDays.Float())
			result := commands.GetRateHistory(ctx, cl, from, to, days, "day")
			data.RateHistory = &result
			c.Sync()
		})

		c.View(func() H {
			currencies := []string{"USD", "EUR", "GBP", "JPY", "CHF", "AUD", "CAD"}
			fromOpts := append([]H{fromCurrency.Bind()}, renderCurrencyOptions(currencies)...)
			toOpts := append([]H{toCurrency.Bind()}, renderCurrencyOptions(currencies)...)

			// Show login UI for OAuth mode when not logged in
			if authMode == "oauth" && !data.LoggedIn {
				return Main(Class("container"),
					Section(
						H1(Text("Wise Account Dashboard")),
						P(Text("Connect your Wise account to get started")),
					),
					Section(
						A(Href(data.AuthURL), Class("button"),
							Text("Connect with Wise"),
						),
						P(Small(Text("You'll be redirected to Wise to authorize access"))),
					),
				)
			}

			historyFromOpts := append([]H{historyFrom.Bind()}, renderCurrencyOptions(currencies)...)
			historyToOpts := append([]H{historyTo.Bind()}, renderCurrencyOptions(currencies)...)

			return Main(Class("container"),
				Section(
					H1(Text("Wise Account Dashboard")),
					P(Text("Manage your Wise account with live data")),
					renderAuthStatus(data),
				),

				Section(
					H2(Text("Profiles")),
					Button(Text("Load Profiles"), refreshProfiles.OnClick()),
					renderProfiles(data.Profiles),
				),

				Section(
					H2(Text("Account Balances")),
					Button(Text("Refresh Balances"), refreshBalances.OnClick()),
					renderBalances(data.Balances),
				),

				Section(
					H2(Text("Exchange Rates")),
					Button(Text("Refresh Rates"), refreshRates.OnClick()),
					renderRates(data.Rates),
				),

				Section(
					H2(Text("Get Quote")),
					Div(Class("grid"),
						Div(
							Label(Text("Amount")),
							Input(Type("number"), amount.Bind()),
						),
						Div(
							Label(Text("From")),
							Select(fromOpts...),
						),
						Div(
							Label(Text("To")),
							Select(toOpts...),
						),
					),
					Button(Text("Get Quote"), getQuote.OnClick()),
					renderQuote(data.Quote),
				),

				Section(
					H2(Text("Transaction Statements")),
					Div(Class("grid"),
						Div(
							Label(Text("Days")),
							Input(Type("number"), statementDays.Bind()),
						),
					),
					Button(Text("Load Statements"), refreshStatements.OnClick()),
					renderStatements(data.Statements),
				),

				Section(
					H2(Text("Rate History")),
					Div(Class("grid"),
						Div(
							Label(Text("From")),
							Select(historyFromOpts...),
						),
						Div(
							Label(Text("To")),
							Select(historyToOpts...),
						),
						Div(
							Label(Text("Days")),
							Input(Type("number"), historyDays.Bind()),
						),
					),
					Button(Text("Get Rate History"), getRateHistory.OnClick()),
					renderRateHistory(data.RateHistory),
				),
			)
		})
	})

	fmt.Printf("Starting Wise Dashboard at http://localhost:%s\n", port)
	fmt.Printf("Auth mode: %s\n", authMode)
	v.Start()
}

func renderAuthStatus(data *AppData) H {
	if data.AuthMode == "token" {
		return P(Small(Text("Authenticated via API token")))
	}
	if data.LoggedIn {
		return P(Small(Text("Connected via OAuth")))
	}
	return nil
}

func renderCurrencyOptions(currencies []string) []H {
	var opts []H
	for _, cur := range currencies {
		opts = append(opts, Option(Value(cur), Text(cur)))
	}
	return opts
}

func renderBalances(balances []commands.BalanceResult) H {
	if len(balances) == 0 {
		return P(Text("Click 'Refresh Balances' to load account balances"))
	}

	var rows []H
	for _, b := range balances {
		if b.Error != nil {
			rows = append(rows, Tr(Td(Textf("Profile %d", b.ProfileID)), Td(Text("Error")), Td(Text(b.Error.Error()))))
			continue
		}
		for _, bal := range b.Balances {
			rows = append(rows, Tr(
				Td(Textf("Profile %d (%s)", b.ProfileID, b.ProfileType)),
				Td(Text(bal.Currency)),
				Td(Strong(Textf("%.2f", bal.Amount))),
			))
		}
	}

	return Table(
		THead(Tr(Th(Text("Profile")), Th(Text("Currency")), Th(Text("Balance")))),
		TBody(rows...),
	)
}

func renderRates(rates []commands.RateResult) H {
	if len(rates) == 0 {
		return P(Text("Click 'Refresh Rates' to load exchange rates"))
	}

	sort.Slice(rates, func(i, j int) bool {
		return rates[i].From+rates[i].To < rates[j].From+rates[j].To
	})

	var rows []H
	for _, r := range rates {
		if r.Error != nil {
			rows = append(rows, Tr(Td(Textf("%s/%s", r.From, r.To)), Td(Text("Error"))))
			continue
		}
		rows = append(rows, Tr(
			Td(Textf("%s/%s", r.From, r.To)),
			Td(Textf("%.6f", r.Rate)),
		))
	}

	return Table(
		THead(Tr(Th(Text("Pair")), Th(Text("Rate")))),
		TBody(rows...),
	)
}

func renderQuote(quote *commands.QuoteResult) H {
	if quote == nil {
		return P(Text("Click 'Get Quote' to get a conversion quote"))
	}

	if quote.Error != nil {
		return P(Style("color: red;"), Text(quote.Error.Error()))
	}

	return Div(
		P(Strong(Textf("%.2f %s → %.2f %s", quote.SourceAmount, quote.From, quote.TargetAmount, quote.To))),
		P(Small(Textf("Rate: %.6f", quote.Rate))),
		P(Small(Textf("Quote ID: %s", quote.QuoteID))),
		P(Small(Textf("Expires: %s", quote.Expires))),
	)
}

func renderProfiles(profiles []commands.ProfileResult) H {
	if len(profiles) == 0 {
		return P(Text("Click 'Load Profiles' to view your Wise profiles"))
	}

	var rows []H
	for _, p := range profiles {
		rows = append(rows, Tr(
			Td(Textf("%d", p.ID)),
			Td(Text(p.Type)),
		))
	}

	return Table(
		THead(Tr(Th(Text("Profile ID")), Th(Text("Type")))),
		TBody(rows...),
	)
}

func renderStatements(statements []commands.StatementResult) H {
	if len(statements) == 0 {
		return P(Text("Click 'Load Statements' to view transaction history"))
	}

	var sections []H
	for _, s := range statements {
		if s.Error != nil {
			sections = append(sections, P(Style("color: red;"), Textf("%s: %v", s.Currency, s.Error)))
			continue
		}

		var rows []H
		if len(s.Transactions) == 0 {
			rows = append(rows, Tr(Td(Attr("colspan", "4"), Text("No transactions"))))
		} else {
			for _, t := range s.Transactions {
				rows = append(rows, Tr(
					Td(Text(t.Date)),
					Td(Text(t.Type)),
					Td(Textf("%.2f", t.Amount)),
					Td(Text(t.Currency)),
				))
			}
		}

		sections = append(sections,
			H4(Textf("%s (Balance ID: %d)", s.Currency, s.BalanceID)),
			Table(
				THead(Tr(Th(Text("Date")), Th(Text("Type")), Th(Text("Amount")), Th(Text("Currency")))),
				TBody(rows...),
			),
		)
	}

	return Div(sections...)
}

func renderRateHistory(history *commands.HistoryResult) H {
	if history == nil {
		return P(Text("Click 'Get Rate History' to view historical exchange rates"))
	}

	if history.Error != nil {
		return P(Style("color: red;"), Text(history.Error.Error()))
	}

	var rows []H
	// Show last 10 data points
	start := len(history.DataPoints) - 10
	if start < 0 {
		start = 0
	}
	for _, p := range history.DataPoints[start:] {
		rows = append(rows, Tr(
			Td(Text(p.Time)),
			Td(Textf("%.6f", p.Rate)),
		))
	}

	return Div(
		P(Strong(Textf("%s/%s Rate History", history.From, history.To))),
		P(Small(Textf("Data points: %d | First: %.6f | Last: %.6f | Min: %.6f | Max: %.6f",
			len(history.DataPoints), history.First, history.Last, history.Min, history.Max))),
		Table(
			THead(Tr(Th(Text("Time")), Th(Text("Rate")))),
			TBody(rows...),
		),
	)
}
