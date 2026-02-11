package notification

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rohithbv/tradebot/internal/model"
)

func TestFormatTradeMessage(t *testing.T) {
	trade := model.Trade{
		ID:        "test-123",
		Symbol:    "AAPL",
		Side:      "buy",
		Qty:       5,
		Price:     187.50,
		Total:     937.50,
		Reason:    "RSI oversold + MACD bullish crossover",
		Timestamp: time.Now(),
	}

	msg := formatTradeMessage(trade)

	if !strings.Contains(msg, "BUY") {
		t.Error("expected message to contain BUY")
	}
	if !strings.Contains(msg, "AAPL") {
		t.Error("expected message to contain AAPL")
	}
	if !strings.Contains(msg, "$187.50") {
		t.Error("expected message to contain price $187.50")
	}
	if !strings.Contains(msg, "$937.50") {
		t.Error("expected message to contain total $937.50")
	}
	if !strings.Contains(msg, "RSI oversold") {
		t.Error("expected message to contain reason")
	}
	if !strings.Contains(msg, "\U0001F4C8") {
		t.Error("expected buy message to contain chart increasing emoji")
	}
}

func TestFormatTradeMessage_Sell(t *testing.T) {
	trade := model.Trade{
		Symbol: "TSLA",
		Side:   "sell",
		Qty:    10,
		Price:  250.00,
		Total:  2500.00,
		Reason: "RSI overbought",
	}

	msg := formatTradeMessage(trade)

	if !strings.Contains(msg, "SELL") {
		t.Error("expected message to contain SELL")
	}
	if !strings.Contains(msg, "\U0001F4C9") {
		t.Error("expected sell message to contain chart decreasing emoji")
	}
}

func TestNotifyTrade(t *testing.T) {
	var receivedPayload map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/bottesttoken123/sendMessage") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type, got %s", r.Header.Get("Content-Type"))
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	notifier := NewTelegramNotifier("testtoken123", 12345)
	notifier.baseURL = srv.URL

	trade := model.Trade{
		ID:        "test-456",
		Symbol:    "GOOGL",
		Side:      "buy",
		Qty:       3,
		Price:     150.00,
		Total:     450.00,
		Reason:    "MACD crossover",
		Timestamp: time.Now(),
	}

	notifier.NotifyTrade(trade)

	if receivedPayload == nil {
		t.Fatal("expected payload to be received")
	}

	chatID, ok := receivedPayload["chat_id"].(float64)
	if !ok || int64(chatID) != 12345 {
		t.Errorf("expected chat_id 12345, got %v", receivedPayload["chat_id"])
	}

	text, ok := receivedPayload["text"].(string)
	if !ok {
		t.Fatal("expected text field in payload")
	}
	if !strings.Contains(text, "GOOGL") {
		t.Error("expected text to contain symbol GOOGL")
	}
	if !strings.Contains(text, "BUY") {
		t.Error("expected text to contain BUY")
	}

	parseMode, ok := receivedPayload["parse_mode"].(string)
	if !ok || parseMode != "Markdown" {
		t.Errorf("expected parse_mode Markdown, got %v", receivedPayload["parse_mode"])
	}
}
