package coinglass

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultBase = "https://open-api-v4.coinglass.com"

type Client struct {
	apiKey string
	base   string
	http   *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		base:   DefaultBase,
		http:   &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) Get(ctx context.Context, path string, query string) ([]byte, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("coinglass: API key not configured")
	}
	u := c.base + path
	if query != "" {
		u += "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("CG-API-KEY", c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("coinglass %s: %s", path, string(body))
	}
	// Paid-plan gate returns HTTP 200 with code 401 / "Upgrade plan".
	if planErr := planUpgradeError(body); planErr != nil {
		return nil, planErr
	}
	return body, nil
}

func planUpgradeError(body []byte) error {
	var wrap struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if json.Unmarshal(body, &wrap) != nil {
		return nil
	}
	if wrap.Code == "401" && (wrap.Msg == "Upgrade plan" || wrap.Msg == "API key missing.") {
		return fmt.Errorf("coinglass: %s", wrap.Msg)
	}
	return nil
}

type ContextScore struct {
	Symbol string
	Score  float64
	Note   string
}

// ScoreSymbol returns -1..+1 context score from funding + OI trend proxies.
func (c *Client) ScoreSymbol(ctx context.Context, coin string) (ContextScore, error) {
	out := ContextScore{Symbol: coin, Score: 0, Note: "neutral"}
	if c.apiKey == "" {
		out.Note = "no_api_key"
		return out, nil
	}

	q := "symbol=" + coin + "&interval=1h&limit=5"
	body, err := c.Get(ctx, "/api/futures/funding-rate/history", q)
	if err != nil {
		// No paid plan — context layer disabled; strategy uses neutral context.
		out.Note = "disabled"
		return out, nil
	}
	var wrap struct {
		Code string `json:"code"`
		Data []struct {
			Close float64 `json:"close"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return out, err
	}
	if len(wrap.Data) == 0 {
		return out, nil
	}
	last := wrap.Data[len(wrap.Data)-1].Close
	switch {
	case last > 0.0003:
		out.Score = -0.5
		out.Note = "crowded_positive_funding"
	case last < -0.0003:
		out.Score = -0.5
		out.Note = "crowded_negative_funding"
	default:
		out.Score = 0.2
		out.Note = "funding_ok"
	}
	return out, nil
}
