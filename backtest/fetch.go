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
	"strconv"
	"strings"
	"time"

	"encore.app/binanceex"
	"encore.app/config"
)

const restBase = "https://fapi.binance.com"

type cachedOnlyKey struct{}

type DataStore struct {
	CacheDir string
	Client   *http.Client
}

func NewDataStore(cacheDir string) *DataStore {
	if cacheDir == "" {
		cacheDir = ResolveCacheDir()
	}
	return &DataStore{
		CacheDir: cacheDir,
		Client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// ResolveCacheDir picks the first existing cache folder.
func ResolveCacheDir() string {
	for _, d := range []string{"backtest/backtest/cache", "backtest/cache"} {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			return d
		}
	}
	return "backtest/cache"
}

// BestCacheDays picks the longest available cached history (90 → 60 → requested).
func (s *DataStore) BestCacheDays(requested int) int {
	candidates := []int{90, 60}
	if requested > 0 {
		candidates = append([]int{requested}, candidates...)
	}
	seen := map[int]bool{}
	for _, d := range candidates {
		if d <= 0 || seen[d] {
			continue
		}
		seen[d] = true
		if len(s.ListCached(d)) >= 10 {
			return d
		}
	}
	if requested > 0 {
		return requested
	}
	return 60
}

func (s *DataStore) ListCached(days int) []string {
	suffix := fmt.Sprintf("_5m_%dd.json", days)
	entries, err := os.ReadDir(s.CacheDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, suffix) {
			continue
		}
		sym := strings.TrimSuffix(name, suffix)
		out = append(out, sym)
	}
	sort.Strings(out)
	return out
}

func (s *DataStore) TopSymbolsByVolume(ctx context.Context, topN int) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, restBase+"/fapi/v1/ticker/24hr", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var tickers []struct {
		Symbol      string `json:"symbol"`
		QuoteVolume string `json:"quoteVolume"`
	}
	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, err
	}
	type row struct {
		sym string
		vol float64
	}
	rows := make([]row, 0, len(tickers))
	for _, t := range tickers {
		if !strings.HasSuffix(t.Symbol, "USDT") {
			continue
		}
		v, _ := strconv.ParseFloat(t.QuoteVolume, 64)
		if v <= 0 {
			continue
		}
		rows = append(rows, row{sym: t.Symbol, vol: v})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].vol > rows[j].vol })
	seen := map[string]bool{}
	var out []string
	add := func(sym string) {
		if seen[sym] {
			return
		}
		seen[sym] = true
		out = append(out, sym)
	}
	for _, c := range config.AlwaysInclude {
		add(c)
	}
	for _, r := range rows {
		add(r.sym)
		if len(out) >= topN {
			break
		}
	}
	return out, nil
}

func (s *DataStore) LoadOrFetch(ctx context.Context, symbol string, days int) ([]Bar, error) {
	_ = os.MkdirAll(s.CacheDir, 0o755)
	cachePath := filepath.Join(s.CacheDir, fmt.Sprintf("%s_5m_%dd.json", symbol, days))
	if raw, err := os.ReadFile(cachePath); err == nil {
		var bars []Bar
		if json.Unmarshal(raw, &bars) == nil && len(bars) > 0 {
			return bars, nil
		}
	}
	if ctx.Value(cachedOnlyKey{}) != nil {
		return nil, fmt.Errorf("no cache for %s", symbol)
	}
	end := time.Now().UTC()
	start := end.Add(-time.Duration(days) * 24 * time.Hour)
	bars, err := s.fetchKlines(ctx, symbol, start, end)
	if err != nil {
		return nil, err
	}
	if raw, err := json.Marshal(bars); err == nil {
		_ = os.WriteFile(cachePath, raw, 0o644)
	}
	return bars, nil
}

func (s *DataStore) fetchKlines(ctx context.Context, symbol string, start, end time.Time) ([]Bar, error) {
	const limit = 1500
	var all []Bar
	cursor := start.UnixMilli()
	endMs := end.UnixMilli()
	for cursor < endMs {
		u := fmt.Sprintf("%s/fapi/v1/klines?symbol=%s&interval=5m&limit=%d&startTime=%d&endTime=%d",
			restBase, symbol, limit, cursor, endMs)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := s.Client.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("klines %s: %s", symbol, string(body))
		}
		var raw [][]interface{}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			break
		}
		for _, row := range raw {
			b, ok := parseKlineRow(row)
			if !ok {
				continue
			}
			all = append(all, b)
		}
		lastOpen := int64(all[len(all)-1].OpenTime.UnixMilli())
		next := lastOpen + 5*60*1000
		if next <= cursor {
			break
		}
		cursor = next
		time.Sleep(80 * time.Millisecond)
	}
	return all, nil
}

func parseKlineRow(row []interface{}) (Bar, bool) {
	if len(row) < 11 {
		return Bar{}, false
	}
	openMs, _ := row[0].(float64)
	closeMs, _ := row[6].(float64)
	return Bar{
		OpenTime:    time.UnixMilli(int64(openMs)).UTC(),
		CloseTime:   time.UnixMilli(int64(closeMs)).UTC(),
		Open:        binanceex.ParseFloat(fmt.Sprint(row[1])),
		High:        binanceex.ParseFloat(fmt.Sprint(row[2])),
		Low:         binanceex.ParseFloat(fmt.Sprint(row[3])),
		Close:       binanceex.ParseFloat(fmt.Sprint(row[4])),
		Volume:      binanceex.ParseFloat(fmt.Sprint(row[5])),
		QuoteVol:    binanceex.ParseFloat(fmt.Sprint(row[7])),
		TakerBuyVol: binanceex.ParseFloat(fmt.Sprint(row[10])),
	}, true
}
