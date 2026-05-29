package binanceex

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	key        string
	secret     string
	net        Network
	httpClient *http.Client
}

func NewClient(key, secret string, testnet bool) *Client {
	return &Client{
		key:        key,
		secret:     secret,
		net:        NetworkForTestnet(testnet),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type Ticker24hr struct {
	Symbol      string `json:"symbol"`
	LastPrice   string `json:"lastPrice"`
	QuoteVolume string `json:"quoteVolume"`
}

type ExchangeInfo struct {
	Symbols []struct {
		Symbol            string `json:"symbol"`
		Status            string `json:"status"`
		ContractType      string `json:"contractType"`
		QuoteAsset        string `json:"quoteAsset"`
		PricePrecision    int    `json:"pricePrecision"`
		QuantityPrecision int    `json:"quantityPrecision"`
		Filters           []struct {
			FilterType string `json:"filterType"`
			StepSize   string `json:"stepSize"`
			MinQty     string `json:"minQty"`
			TickSize   string `json:"tickSize"`
		} `json:"filters"`
	} `json:"symbols"`
}

type AccountBalance struct {
	TotalWalletBalance string `json:"totalWalletBalance"`
	AvailableBalance   string `json:"availableBalance"`
}

type PositionRisk struct {
	Symbol           string `json:"symbol"`
	PositionAmt      string `json:"positionAmt"`
	EntryPrice       string `json:"entryPrice"`
	UnRealizedProfit string `json:"unRealizedProfit"`
	Leverage         string `json:"leverage"`
}

type OrderResponse struct {
	OrderID       int64  `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	Status        string `json:"status"`
	AvgPrice      string `json:"avgPrice"`
	ExecutedQty   string `json:"executedQty"`
}

func (c *Client) Get(ctx context.Context, path string, signed bool, params url.Values) ([]byte, error) {
	if params == nil {
		params = url.Values{}
	}
	if signed {
		params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
		params.Set("recvWindow", "5000")
		mac := hmac.New(sha256.New, []byte(c.secret))
		mac.Write([]byte(params.Encode()))
		params.Set("signature", hex.EncodeToString(mac.Sum(nil)))
	}
	u := c.net.RESTBase + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if signed && c.key != "" {
		req.Header.Set("X-MBX-APIKEY", c.key)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("binance GET %s: %s %s", path, resp.Status, string(body))
	}
	return body, nil
}

func (c *Client) Post(ctx context.Context, path string, params url.Values) ([]byte, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("recvWindow", "5000")
	mac := hmac.New(sha256.New, []byte(c.secret))
	mac.Write([]byte(params.Encode()))
	params.Set("signature", hex.EncodeToString(mac.Sum(nil)))

	u := c.net.RESTBase + path + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", c.key)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("binance POST %s: %s %s", path, resp.Status, string(body))
	}
	return body, nil
}

func (c *Client) Delete(ctx context.Context, path string, params url.Values) ([]byte, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("recvWindow", "5000")
	mac := hmac.New(sha256.New, []byte(c.secret))
	mac.Write([]byte(params.Encode()))
	params.Set("signature", hex.EncodeToString(mac.Sum(nil)))
	u := c.net.RESTBase + path + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", c.key)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("binance DELETE %s: %s %s", path, resp.Status, string(body))
	}
	return body, nil
}

func (c *Client) Ticker24hrAll(ctx context.Context) ([]Ticker24hr, error) {
	body, err := c.Get(ctx, "/fapi/v1/ticker/24hr", false, nil)
	if err != nil {
		return nil, err
	}
	var out []Ticker24hr
	return out, json.Unmarshal(body, &out)
}

func (c *Client) ExchangeInfo(ctx context.Context) (*ExchangeInfo, error) {
	body, err := c.Get(ctx, "/fapi/v1/exchangeInfo", false, nil)
	if err != nil {
		return nil, err
	}
	var out ExchangeInfo
	return &out, json.Unmarshal(body, &out)
}

func (c *Client) Account(ctx context.Context) (*AccountBalance, error) {
	body, err := c.Get(ctx, "/fapi/v2/account", true, nil)
	if err != nil {
		return nil, err
	}
	var raw struct {
		TotalWalletBalance string `json:"totalWalletBalance"`
		AvailableBalance   string `json:"availableBalance"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return &AccountBalance{
		TotalWalletBalance: raw.TotalWalletBalance,
		AvailableBalance:   raw.AvailableBalance,
	}, nil
}

func (c *Client) PositionRisk(ctx context.Context, symbol string) ([]PositionRisk, error) {
	p := url.Values{}
	if symbol != "" {
		p.Set("symbol", symbol)
	}
	body, err := c.Get(ctx, "/fapi/v2/positionRisk", true, p)
	if err != nil {
		return nil, err
	}
	var out []PositionRisk
	return out, json.Unmarshal(body, &out)
}

func (c *Client) PlaceOrder(ctx context.Context, p url.Values) (*OrderResponse, error) {
	body, err := c.Post(ctx, "/fapi/v1/order", p)
	if err != nil {
		return nil, err
	}
	var out OrderResponse
	return &out, json.Unmarshal(body, &out)
}

func (c *Client) CancelOrder(ctx context.Context, symbol string, orderID int64) error {
	p := url.Values{}
	p.Set("symbol", symbol)
	p.Set("orderId", strconv.FormatInt(orderID, 10))
	_, err := c.Delete(ctx, "/fapi/v1/order", p)
	return err
}

func (c *Client) SetLeverage(ctx context.Context, symbol string, lev int) error {
	p := url.Values{}
	p.Set("symbol", symbol)
	p.Set("leverage", strconv.Itoa(lev))
	_, err := c.Post(ctx, "/fapi/v1/leverage", p)
	return err
}

func ParseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

func FormatQty(q float64, precision int) string {
	format := "%." + strconv.Itoa(precision) + "f"
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf(format, q), "0"), ".")
}
