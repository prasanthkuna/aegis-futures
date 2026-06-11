package backtest

import (
	"sort"
	"time"

	"encore.app/config"
)

func RollingQuoteVol24h(bars []Bar, idx int) float64 {
	if idx < 0 || idx >= len(bars) {
		return 0
	}
	const barsPerDay = 288
	start := idx - barsPerDay + 1
	if start < 0 {
		start = 0
	}
	var sum float64
	for i := start; i <= idx; i++ {
		sum += bars[i].QuoteVol
	}
	return sum
}

func UniverseAt(all map[string][]Bar, idxBySym map[string]int, topN int, at time.Time) []string {
	type row struct {
		sym string
		vol float64
	}
	var rows []row
	for sym, bars := range all {
		idx := idxBySym[sym]
		if idx < 0 || idx >= len(bars) {
			continue
		}
		if bars[idx].CloseTime.After(at) {
			continue
		}
		vol := RollingQuoteVol24h(bars, idx)
		if vol > 0 {
			rows = append(rows, row{sym: sym, vol: vol})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].vol != rows[j].vol {
			return rows[i].vol > rows[j].vol
		}
		return rows[i].sym < rows[j].sym
	})
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
	return out
}

func BuildTimeline(all map[string][]Bar) []time.Time {
	seen := map[int64]bool{}
	var ts []int64
	for _, bars := range all {
		for _, b := range bars {
			ms := b.CloseTime.UnixMilli()
			if !seen[ms] {
				seen[ms] = true
				ts = append(ts, ms)
			}
		}
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
	out := make([]time.Time, len(ts))
	for i, ms := range ts {
		out[i] = time.UnixMilli(ms).UTC()
	}
	return out
}

func IndexMap(all map[string][]Bar) map[string]map[int64]int {
	out := make(map[string]map[int64]int, len(all))
	for sym, bars := range all {
		m := make(map[int64]int, len(bars))
		for i, b := range bars {
			m[b.CloseTime.UnixMilli()] = i
		}
		out[sym] = m
	}
	return out
}
