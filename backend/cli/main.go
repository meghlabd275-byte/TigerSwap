package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"tigerswap/backend/go/sdk"
)

func main() {
	// Parse flags
	apiURL := flag.String("api", "https://api.tigerswap.io", "API URL")
	apiKey := flag.String("key", os.Getenv("TIGERSWAP_API_KEY"), "API Key")
	chainID := flag.Int64("chain", 1, "Chain ID")
	flag.Parse()

	// Create client
	client := tigerswap.NewClient(*apiURL, *apiKey)
	ctx := context.Background()

	// Get command
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "chains":
		chains, err := client.GetChains(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		printChains(chains)

	case "tokens":
		tokens, err := client.GetTokens(ctx, *chainID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		printTokens(tokens)

	case "quote":
		if len(os.Args) < 6 {
			fmt.Println("Usage: tigerswap quote <from_token> <to_token> <amount>")
			os.Exit(1)
		}
		quote, err := client.GetQuote(ctx, os.Args[2], os.Args[3], os.Args[4], *chainID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		printQuote(quote)

	case "stats":
		stats, err := client.GetMarketStats(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		printStats(stats)

	case "pools":
		pools, err := client.GetPools(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		printPools(pools)

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("TigerSwap CLI")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  tigerswap [options] <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  chains              - List supported chains")
	fmt.Println("  tokens              - List tokens for chain")
	fmt.Println("  quote <from> <to> <amount> - Get swap quote")
	fmt.Println("  stats               - Get market statistics")
	fmt.Println("  pools               - List liquidity pools")
	fmt.Println("")
	fmt.Println("Options:")
	flag.PrintDefaults()
}

func printChains(chains []tigerswap.Chain) {
	fmt.Println("Supported Chains:")
	fmt.Println("-----------------")
	for _, c := range chains {
		fmt.Printf("%d: %s (%s) - %s\n", c.ID, c.Name, c.NativeSymbol, c.ChainType)
	}
}

func printTokens(tokens []tigerswap.Token) {
	fmt.Println("Tokens:")
	fmt.Println("-------")
	for _, t := range tokens {
		price := "n/a"
		if t.PriceUSD != nil {
			price = fmt.Sprintf("$%.2f", *t.PriceUSD)
		}
		fmt.Printf("%s: %s (%s) - %s\n", t.Symbol, t.Name, t.Address, price)
	}
}

func printQuote(quote *tigerswap.SwapQuote) {
	fmt.Println("Swap Quote:")
	fmt.Println("-----------")
	fmt.Printf("Input: %s\n", quote.AmountIn)
	fmt.Printf("Output: %s\n", quote.AmountOut)
	fmt.Printf("Min Output: %s\n", quote.AmountOutMin)
	fmt.Printf("Price Impact: %.4f\n", quote.PriceImpact)
	fmt.Printf("Route: %s\n", strings.Join(quote.Route, " -> "))
}

func printStats(stats *tigerswap.MarketStats) {
	fmt.Println("Market Statistics:")
	fmt.Println("------------------")
	fmt.Printf("Total TVL: $%.2f\n", stats.TotalTVL)
	fmt.Printf("24h Volume: $%.2f\n", stats.Volume24h)
	fmt.Printf("Pairs: %d\n", stats.TotalPairs)
	fmt.Printf("Users: %d\n", stats.TotalUsers)
}

func printPools(pools []tigerswap.Pool) {
	fmt.Println("Liquidity Pools:")
	fmt.Println("----------------")
	for _, p := range pools {
		fmt.Printf("%s/%s: %.6f / %.6f (fee %d bps)\n",
			p.Symbol0, p.Symbol1, p.Reserve0, p.Reserve1, p.FeeBps)
	}
}

// Add JSON printing for debugging
func printJSON(v interface{}) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
