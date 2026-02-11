package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rohithbv/tradebot/internal/model"
)

// Notifier sends trade notifications.
type Notifier interface {
	NotifyTrade(trade model.Trade)
}

// TelegramNotifier sends trade notifications to a Telegram chat.
type TelegramNotifier struct {
	botToken string
	chatID   int64
	client   *http.Client
	baseURL  string
}

// NewTelegramNotifier creates a new TelegramNotifier.
func NewTelegramNotifier(botToken string, chatID int64) *TelegramNotifier {
	return &TelegramNotifier{
		botToken: botToken,
		chatID:   chatID,
		client:   &http.Client{Timeout: 10 * time.Second},
		baseURL:  "https://api.telegram.org",
	}
}

// NotifyTrade sends a trade notification to Telegram.
func (t *TelegramNotifier) NotifyTrade(trade model.Trade) {
	msg := formatTradeMessage(trade)

	payload := map[string]interface{}{
		"chat_id":    t.chatID,
		"text":       msg,
		"parse_mode": "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal telegram payload", "error", err)
		return
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, t.botToken)
	resp, err := t.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error("failed to send telegram notification", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("telegram API returned non-200", "status", resp.StatusCode)
	}
}

func formatTradeMessage(t model.Trade) string {
	emoji := "\U0001F4C8" // chart increasing
	if strings.EqualFold(t.Side, "sell") {
		emoji = "\U0001F4C9" // chart decreasing
	}

	return fmt.Sprintf(
		"%s *%s %s*\nQty: `%d` @ `$%.2f`\nTotal: `$%.2f`\nReason: _%s_",
		emoji,
		strings.ToUpper(t.Side),
		t.Symbol,
		t.Qty,
		t.Price,
		t.Total,
		t.Reason,
	)
}
