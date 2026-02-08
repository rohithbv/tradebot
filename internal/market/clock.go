package market

import (
	"fmt"
	"time"
)

// ClockInfo contains the current market clock state.
type ClockInfo struct {
	IsOpen     bool
	NextChange time.Time // next open if closed, next close if open
}

// GetClock retrieves the current market clock from the Alpaca trading API.
func (mc *MarketClient) GetClock() (*ClockInfo, error) {
	clock, err := mc.tradeCli.GetClock()
	if err != nil {
		return nil, fmt.Errorf("fetching market clock: %w", err)
	}

	var nextChange time.Time
	if clock.IsOpen {
		nextChange = clock.NextClose
	} else {
		nextChange = clock.NextOpen
	}

	return &ClockInfo{
		IsOpen:     clock.IsOpen,
		NextChange: nextChange,
	}, nil
}
