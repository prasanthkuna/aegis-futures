package binanceex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// KlineBar is one closed 5m futures candle.
type KlineBar struct {
	OpenTime  time.Time
	CloseTime time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// FetchKlines5m loads historical 5m klines (newest `limit` bars, max 1500 per call).
func (c *Client) FetchKlines5m(ctx context.Context, symbol string, limit int) ([]KlineBar, error) {
	if c == nil {
		return nil, fmt.Errorf("nil client")
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 1500 {
		limit = 1500
	}
	p := url.Values{}
	p.Set("symbol", symbol)
	p.Set("interval", "5m")
	p.Set("limit", strconv.Itoa(limit))
	body, err := c.Get(ctx, "/fapi/v1/klines", false, p)
	if err != nil {
		return nil, err
	}
	var raw [][]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]KlineBar, 0, len(raw))
	for _, row := range raw {
		if len(row) < 7 {
			continue
		}
		openMs, _ := row[0].(float64)
		closeMs, _ := row[6].(float64)
		out = append(out, KlineBar{
			OpenTime:  time.UnixMilli(int64(openMs)).UTC(),
			CloseTime: time.UnixMilli(int64(closeMs)).UTC(),
			Open:      ParseFloat(fmt.Sprint(row[1])),
			High:      ParseFloat(fmt.Sprint(row[2])),
			Low:       ParseFloat(fmt.Sprint(row[3])),
			Close:     ParseFloat(fmt.Sprint(row[4])),
			Volume:    ParseFloat(fmt.Sprint(row[5])),
		})
	}
	return out, nil
}

// FetchKlines5mHistory pages backward until `bars` count or start time.
func (c *Client) FetchKlines5mHistory(ctx context.Context, symbol string, bars int) ([]KlineBar, error) {
	if bars <= 1500 {
		return c.FetchKlines5m(ctx, symbol, bars)
	}
	var all []KlineBar
	endMs := time.Now().UTC().UnixMilli()
	for len(all) < bars {
		p := url.Values{}
		p.Set("symbol", symbol)
		p.Set("interval", "5m")
		p.Set("limit", "1500")
		p.Set("endTime", strconv.FormatInt(endMs, 10))
		body, err := c.Get(ctx, "/fapi/v1/klines", false, p)
		if err != nil {
			return nil, err
		}
		var raw [][]interface{}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			break
		}
		chunk := make([]KlineBar, 0, len(raw))
		for _, row := range raw {
			if len(row) < 7 {
				continue
			}
			openMs, _ := row[0].(float64)
			closeMs, _ := row[6].(float64)
			chunk = append(chunk, KlineBar{
				OpenTime:  time.UnixMilli(int64(openMs)).UTC(),
				CloseTime: time.UnixMilli(int64(closeMs)).UTC(),
				Open:      ParseFloat(fmt.Sprint(row[1])),
				High:      ParseFloat(fmt.Sprint(row[2])),
				Low:       ParseFloat(fmt.Sprint(row[3])),
				Close:     ParseFloat(fmt.Sprint(row[4])),
				Volume:    ParseFloat(fmt.Sprint(row[5])),
			})
		}
		if len(chunk) == 0 {
			break
		}
		all = append(chunk, all...)
		endMs = chunk[0].OpenTime.UnixMilli() - 1
		if len(chunk) < 1500 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(80 * time.Millisecond):
		}
	}
	if len(all) > bars {
		all = all[len(all)-bars:]
	}
	return all, nil
}
