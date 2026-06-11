package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"encore.app/binanceex"
)

const futuresDataBase = "https://fapi.binance.com/futures/data"

// ContextPoint is Binance derivatives context aligned to a 5m bucket.
type ContextPoint struct {
	Time              time.Time `json:"time"`
	OpenInterest      float64   `json:"openInterest"`
	OIDeltaPct        float64   `json:"oiDeltaPct"`
	FundingRate       float64   `json:"fundingRate"`
	TakerBuySellRatio float64   `json:"takerBuySellRatio"`
	LongShortRatio    float64   `json:"longShortRatio"`
}

// ContextSeries is per-symbol context history keyed by 5m open ms.
type ContextSeries map[int64]ContextPoint

func (s *DataStore) contextPath(symbol string, days int) string {
	return filepath.Join(s.CacheDir, fmt.Sprintf("%s_ctx_%dd.json", symbol, days))
}

// FetchAndCacheContext pulls OI, funding, taker L/S, global L/S from Binance (free).
func (s *DataStore) FetchAndCacheContext(ctx context.Context, symbol string, days int) error {
	if s.Client == nil {
		s.Client = &http.Client{Timeout: 30 * time.Second}
	}
	end := time.Now().UTC()
	start := end.Add(-time.Duration(days) * 24 * time.Hour)
	// API caps OI/taker at ~30d; still merge what we can onto 60d klines.
	apiStart := end.Add(-30 * 24 * time.Hour)
	if start.Before(apiStart) {
		start = apiStart
	}

	oi, err := fetchOIHist(ctx, s.Client, symbol, start, end)
	if err != nil {
		return fmt.Errorf("%s oi: %w", symbol, err)
	}
	taker, err := fetchTakerRatio(ctx, s.Client, symbol, start, end)
	if err != nil {
		return fmt.Errorf("%s taker: %w", symbol, err)
	}
	ls, err := fetchLongShort(ctx, s.Client, symbol, start, end)
	if err != nil {
		return fmt.Errorf("%s ls: %w", symbol, err)
	}
	fund, err := fetchFunding(ctx, s.Client, symbol, end.Add(-60*24*time.Hour), end)
	if err != nil {
		return fmt.Errorf("%s funding: %w", symbol, err)
	}

	series := mergeContext(oi, taker, ls, fund)
	_ = os.MkdirAll(s.CacheDir, 0o755)
	raw, err := json.Marshal(seriesToSlice(series))
	if err != nil {
		return err
	}
	return os.WriteFile(s.contextPath(symbol, days), raw, 0o644)
}

func (s *DataStore) LoadContext(symbol string, days int) (ContextSeries, error) {
	raw, err := os.ReadFile(s.contextPath(symbol, days))
	if err != nil {
		return nil, err
	}
	var pts []ContextPoint
	if err := json.Unmarshal(raw, &pts); err != nil {
		return nil, err
	}
	out := make(ContextSeries, len(pts))
	for _, p := range pts {
		out[p.Time.UnixMilli()] = p
	}
	return out, nil
}

func (s *DataStore) FetchContextBatch(ctx context.Context, symbols []string, days int) (int, error) {
	ok := 0
	for i, sym := range symbols {
		if err := s.FetchAndCacheContext(ctx, sym, days); err != nil {
			fmt.Printf("  ctx %s: %v\n", sym, err)
			continue
		}
		ok++
		if (i+1)%10 == 0 {
			fmt.Printf("  context %d/%d ok\n", i+1, len(symbols))
		}
		time.Sleep(120 * time.Millisecond)
	}
	return ok, nil
}

func seriesToSlice(m ContextSeries) []ContextPoint {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([]ContextPoint, 0, len(keys))
	for _, k := range keys {
		out = append(out, m[k])
	}
	return out
}

func mergeContext(oi, taker, ls []tsPoint, fund []fundPoint) ContextSeries {
	out := ContextSeries{}
	for _, p := range oi {
		cp := out[p.Ts]
		cp.Time = time.UnixMilli(p.Ts).UTC()
		cp.OpenInterest = p.Val
		out[p.Ts] = cp
	}
	// OI delta vs 3 bars (15m)
	keys := sortedKeys(out)
	for i, k := range keys {
		if i < 3 {
			continue
		}
		prev := out[keys[i-3]].OpenInterest
		cur := out[k]
		if prev > 0 {
			cur.OIDeltaPct = (cur.OpenInterest - prev) / prev * 100
		}
		out[k] = cur
	}
	for _, p := range taker {
		cp := out[p.Ts]
		if cp.Time.IsZero() {
			cp.Time = time.UnixMilli(p.Ts).UTC()
		}
		cp.TakerBuySellRatio = p.Val
		out[p.Ts] = cp
	}
	for _, p := range ls {
		cp := out[p.Ts]
		if cp.Time.IsZero() {
			cp.Time = time.UnixMilli(p.Ts).UTC()
		}
		cp.LongShortRatio = p.Val
		out[p.Ts] = cp
	}
	// forward-fill funding onto 5m buckets
	if len(fund) == 0 {
		return out
	}
	fi := 0
	for _, k := range sortedKeys(out) {
		for fi+1 < len(fund) && fund[fi+1].Ts <= k {
			fi++
		}
		cp := out[k]
		cp.FundingRate = fund[fi].Val
		out[k] = cp
	}
	return out
}

func sortedKeys(m ContextSeries) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

type tsPoint struct {
	Ts  int64
	Val float64
}

type fundPoint struct {
	Ts  int64
	Val float64
}

func fetchOIHist(ctx context.Context, c *http.Client, symbol string, start, end time.Time) ([]tsPoint, error) {
	return paginateFuturesData(ctx, c, "/openInterestHist", symbol, start, end, func(m map[string]interface{}) (int64, float64, bool) {
		ts, _ := m["timestamp"].(float64)
		v := binanceex.ParseFloat(fmt.Sprint(m["sumOpenInterest"]))
		return int64(ts), v, ts > 0
	})
}

func fetchTakerRatio(ctx context.Context, c *http.Client, symbol string, start, end time.Time) ([]tsPoint, error) {
	return paginateFuturesData(ctx, c, "/takerlongshortRatio", symbol, start, end, func(m map[string]interface{}) (int64, float64, bool) {
		ts, _ := m["timestamp"].(float64)
		v := binanceex.ParseFloat(fmt.Sprint(m["buySellRatio"]))
		return int64(ts), v, ts > 0
	})
}

func fetchLongShort(ctx context.Context, c *http.Client, symbol string, start, end time.Time) ([]tsPoint, error) {
	return paginateFuturesData(ctx, c, "/globalLongShortAccountRatio", symbol, start, end, func(m map[string]interface{}) (int64, float64, bool) {
		ts, _ := m["timestamp"].(float64)
		v := binanceex.ParseFloat(fmt.Sprint(m["longShortRatio"]))
		return int64(ts), v, ts > 0
	})
}

func paginateFuturesData(ctx context.Context, c *http.Client, path, symbol string, start, end time.Time, parse func(map[string]interface{}) (int64, float64, bool)) ([]tsPoint, error) {
	var all []tsPoint
	cursor := start.UnixMilli()
	endMs := end.UnixMilli()
	for cursor < endMs {
		u := fmt.Sprintf("%s%s?symbol=%s&period=5m&limit=500&startTime=%d&endTime=%d",
			futuresDataBase, path, symbol, cursor, endMs)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("%s: %s", path, string(body))
		}
		var rows []map[string]interface{}
		if err := json.Unmarshal(body, &rows); err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		var lastTs int64
		for _, row := range rows {
			ts, val, ok := parse(row)
			if !ok {
				continue
			}
			all = append(all, tsPoint{Ts: ts, Val: val})
			lastTs = ts
		}
		next := lastTs + 5*60*1000
		if next <= cursor {
			break
		}
		cursor = next
		time.Sleep(80 * time.Millisecond)
	}
	return all, nil
}

func fetchFunding(ctx context.Context, c *http.Client, symbol string, start, end time.Time) ([]fundPoint, error) {
	u := fmt.Sprintf("%s/fapi/v1/fundingRate?symbol=%s&startTime=%d&endTime=%d&limit=1000",
		restBase, symbol, start.UnixMilli(), end.UnixMilli())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("funding: %s", string(body))
	}
	var rows []struct {
		FundingTime int64  `json:"fundingTime"`
		FundingRate string `json:"fundingRate"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, err
	}
	out := make([]fundPoint, 0, len(rows))
	for _, r := range rows {
		out = append(out, fundPoint{Ts: r.FundingTime, Val: binanceex.ParseFloat(r.FundingRate)})
	}
	return out, nil
}

// ContextScoreFromState maps Binance context to -1..+1 (replaces CoinGlass in backtest).
func ContextScoreFromState(oiDelta, funding, takerBSR float64) float64 {
	score := 0.0
	if funding > 0.0003 {
		score -= 0.4
	} else if funding < -0.0003 {
		score -= 0.4
	} else {
		score += 0.15
	}
	if oiDelta < -3 {
		score += 0.35 // flush often precedes exhaustion fade
	} else if oiDelta > 3 {
		score += 0.1
	}
	if takerBSR > 1.3 {
		score += 0.15
	} else if takerBSR < 0.7 {
		score -= 0.1
	}
	if score > 1 {
		return 1
	}
	if score < -1 {
		return -1
	}
	return score
}

func LookupContext(series ContextSeries, t time.Time) (ContextPoint, bool) {
	if len(series) == 0 {
		return ContextPoint{}, false
	}
	ms := t.UnixMilli()
	if p, ok := series[ms]; ok {
		return p, true
	}
	// nearest 5m bucket
	bucket := (ms / (5 * 60 * 1000)) * (5 * 60 * 1000)
	p, ok := series[bucket]
	return p, ok
}

// TopCachedSymbols returns first n symbols from cache (BTC/ETH prioritized).
func (s *DataStore) HasContextBatch(days, minSymbols int) (bool, int) {
	n := 0
	for _, sym := range s.ListCached(days) {
		if _, err := s.LoadContext(sym, days); err == nil {
			n++
		}
	}
	return n >= minSymbols, n
}

func (s *DataStore) TopCachedSymbols(days, n int) []string {
	all := s.ListCached(days)
	priority := map[string]int{"BTCUSDT": 0, "ETHUSDT": 1, "BNBUSDT": 2, "SOLUSDT": 3}
	sort.SliceStable(all, func(i, j int) bool {
		pi, pj := priority[all[i]], priority[all[j]]
		if pi != pj {
			return pi < pj
		}
		return all[i] < all[j]
	})
	if n > 0 && len(all) > n {
		return all[:n]
	}
	return all
}
