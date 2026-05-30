package alerts

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Telegram struct {
	Token  string
	ChatID string
	http   *http.Client
}

func NewTelegram(token, chatID string) *Telegram {
	return &Telegram{Token: token, ChatID: chatID, http: &http.Client{Timeout: 10 * time.Second}}
}

func (t *Telegram) Send(ctx context.Context, alertType, body string) error {
	if t == nil || t.Token == "" || t.ChatID == "" {
		return nil
	}
	msg := fmt.Sprintf("Aegis Futures Alert\nType: %s\n%s\nTime: %s UTC",
		alertType, body, time.Now().UTC().Format("2006-01-02 15:04"))
	api := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.Token)
	form := url.Values{}
	form.Set("chat_id", t.ChatID)
	form.Set("text", msg)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram status %s", resp.Status)
	}
	return nil
}
