package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

// Market represents a betting market with game info, bet type, odds, and line
type Market struct {
	Game    string  // Game description (e.g., "Team A vs Team B")
	Side    string  // Side of the bet (e.g., "over", "under")
	Odds    float64 // American odds
	Line    float64 // Point spread or total line
	BetType string  // Type of bet ("Moneyline", "Total", "Spread")
}

const (
	ScrapeInterval = 30 * time.Second
	PageTimeout    = 2 * time.Minute
	NFLURL         = "https://sportsbook.draftkings.com/leagues/football/nfl"
)

var betTypes = []string{"Spread", "Total", "Moneyline"}

func main() {
	log.Println("Starting DraftKings NFL scraper...")
	log.Printf("Scraping every %v\n", ScrapeInterval)

	// Run immediately on start
	scrapeAndLog()

	// Then run continuously
	ticker := time.NewTicker(ScrapeInterval)
	defer ticker.Stop()

	for range ticker.C {
		scrapeAndLog()
	}
}

func scrapeAndLog() {
	log.Println("\n=== Starting scrape ===")

	markets, err := scrapeNFLMarkets()
	if err != nil {
		log.Printf("Error scraping markets: %v\n", err)
		return
	}

	printMarkets(markets)
}

func scrapeNFLMarkets() ([]Market, error) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, PageTimeout)
	defer cancel()

	var htmlContent string

	log.Println("Loading page with headless Chrome...")
	err := chromedp.Run(ctx,
		chromedp.Navigate(NFLURL),
		chromedp.WaitVisible(`.cms-market-selector-content`, chromedp.ByQuery),
		chromedp.OuterHTML(`html`, &htmlContent),
	)
	if err != nil {
		return nil, fmt.Errorf("chromedp error: %w", err)
	}

	log.Println("Parsing markets...")
	return parseMarkets(htmlContent), nil
}

func parseMarkets(html string) []Market {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		log.Printf("Error parsing HTML: %v\n", err)
		return nil
	}

	var markets []Market

	doc.Find(".cb-market__template").Each(func(i int, gameWrapper *goquery.Selection) {
		var teamA, teamB string

		// Extract team names
		gameWrapper.Find(".cb-market__label-inner").Each(func(j int, teamSel *goquery.Selection) {
			switch j {
			case 0:
				teamA = strings.TrimSpace(teamSel.Text())
			case 1:
				teamB = strings.TrimSpace(teamSel.Text())
			}
		})

		if teamA == "" || teamB == "" {
			return
		}

		gameDescription := fmt.Sprintf("%s vs %s", teamA, teamB)

		// Extract market data from buttons
		gameWrapper.Find(".cb-market__button").Each(func(j int, button *goquery.Selection) {
			lineText := button.Find(".cb-market__button-points").Text()
			oddsText := button.Find(".cb-market__button-odds").Text()
			betType := betTypes[j%3]

			line := parseOdds(lineText)
			odds := parseOdds(oddsText)

			// Determine side (first 3 are one side, next 3 are the other)
			side := "over"
			if j >= 3 {
				side = "under"
			}

			market := Market{
				Game:    gameDescription,
				Side:    side,
				Odds:    odds,
				Line:    line,
				BetType: betType,
			}
			markets = append(markets, market)
		})
	})

	return markets
}

func parseOdds(oddsStr string) float64 {
	oddsStr = strings.TrimSpace(oddsStr)
	if oddsStr == "" {
		return 0.0
	}

	// Handle both regular minus (-) and unicode minus (−)
	isMinus := strings.HasPrefix(oddsStr, "-") || strings.HasPrefix(oddsStr, "−")
	oddsStr = strings.TrimPrefix(oddsStr, "+")
	oddsStr = strings.TrimPrefix(oddsStr, "-")
	oddsStr = strings.TrimPrefix(oddsStr, "−")

	val, err := strconv.ParseFloat(oddsStr, 64)
	if err != nil {
		return 0.0
	}

	if isMinus {
		return -val
	}
	return val
}

func printMarkets(markets []Market) {
	if len(markets) == 0 {
		log.Println("No markets found")
		return
	}

	fmt.Printf("\n=== Found %d Markets ===\n\n", len(markets))

	// Group markets by game
	gameMap := make(map[string][]Market)
	for _, m := range markets {
		gameMap[m.Game] = append(gameMap[m.Game], m)
	}

	// Print each game's markets
	for game, gameMarkets := range gameMap {
		fmt.Printf("%s\n", game)
		fmt.Println(strings.Repeat("-", len(game)))

		for _, m := range gameMarkets {
			if m.Line != 0 {
				fmt.Printf("  %-10s | %-8s | Line: %6.1f | Odds: %+6.0f\n",
					m.BetType, m.Side, m.Line, m.Odds)
			} else {
				fmt.Printf("  %-10s | %-8s | Odds: %+6.0f\n",
					m.BetType, m.Side, m.Odds)
			}
		}
		fmt.Println()
	}
}
