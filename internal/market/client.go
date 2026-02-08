package market

import (
	"fmt"
	"strings"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"

	"github.com/rohithbv/tradebot/internal/config"
	"github.com/rohithbv/tradebot/internal/model"
)

// MarketClient wraps the Alpaca market data and trading API clients.
type MarketClient struct {
	dataCli  *marketdata.Client
	tradeCli *alpaca.Client
	feed     marketdata.Feed
}

// NewMarketClient creates a new MarketClient configured from the given AlpacaConfig.
func NewMarketClient(cfg config.AlpacaConfig) *MarketClient {
	dataCli := marketdata.NewClient(marketdata.ClientOpts{
		APIKey:    cfg.APIKey,
		APISecret: cfg.APISecret,
	})

	tradeCli := alpaca.NewClient(alpaca.ClientOpts{
		APIKey:    cfg.APIKey,
		APISecret: cfg.APISecret,
		BaseURL:   cfg.BaseURL,
	})

	feed := parseFeed(cfg.Feed)

	return &MarketClient{
		dataCli:  dataCli,
		tradeCli: tradeCli,
		feed:     feed,
	}
}

// parseFeed converts a feed string from config to the marketdata Feed constant.
func parseFeed(s string) marketdata.Feed {
	switch strings.ToLower(s) {
	case "sip":
		return marketdata.SIP
	case "iex":
		return marketdata.IEX
	default:
		return marketdata.IEX
	}
}

// GetBars retrieves 1-minute OHLCV bars for the given symbols over the specified lookback duration.
func (mc *MarketClient) GetBars(symbols []string, lookback time.Duration) (map[string][]model.Bar, error) {
	if len(symbols) == 0 {
		return map[string][]model.Bar{}, nil
	}

	start := time.Now().Add(-lookback)

	multiBars, err := mc.dataCli.GetMultiBars(symbols, marketdata.GetBarsRequest{
		TimeFrame: marketdata.OneMin,
		Start:     start,
		Feed:      mc.feed,
	})
	if err != nil {
		return nil, fmt.Errorf("fetching multi bars: %w", err)
	}

	result := make(map[string][]model.Bar, len(multiBars))
	for sym, bars := range multiBars {
		converted := make([]model.Bar, len(bars))
		for i, b := range bars {
			converted[i] = model.Bar{
				Symbol:    sym,
				Timestamp: b.Timestamp,
				Open:      b.Open,
				High:      b.High,
				Low:       b.Low,
				Close:     b.Close,
				Volume:    int64(b.Volume),
			}
		}
		result[sym] = converted
	}

	return result, nil
}

// GetLatestPrices returns the most recent close price for each of the given symbols.
func (mc *MarketClient) GetLatestPrices(symbols []string) (map[string]float64, error) {
	if len(symbols) == 0 {
		return map[string]float64{}, nil
	}

	latestBars, err := mc.dataCli.GetLatestBars(symbols, marketdata.GetLatestBarRequest{
		Feed: mc.feed,
	})
	if err != nil {
		return nil, fmt.Errorf("fetching latest bars: %w", err)
	}

	prices := make(map[string]float64, len(latestBars))
	for sym, bar := range latestBars {
		prices[sym] = bar.Close
	}

	return prices, nil
}
