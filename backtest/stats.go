package backtest

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// FoldMetrics is OOS performance inside one walk-forward test window.
type FoldMetrics struct {
	OOSNetPnL float64
	OOSTrades int
	OOSPF     float64
	Passed    bool
}

// OOSMetricsBetween scores trades whose entry falls in [start, end).
func (res *Result) OOSMetricsBetween(start, end time.Time) FoldMetrics {
	var wins, losses float64
	var trades int
	var pnl float64
	for _, tr := range res.Trades {
		if tr.EntryTime.Before(start) || !tr.EntryTime.Before(end) {
			continue
		}
		trades++
		pnl += tr.PnLUSD
		if tr.PnLUSD > 0 {
			wins += tr.PnLUSD
		} else if tr.PnLUSD < 0 {
			losses -= tr.PnLUSD
		}
	}
	m := FoldMetrics{OOSNetPnL: pnl, OOSTrades: trades}
	if losses > 0 {
		m.OOSPF = wins / losses
	} else if wins > 0 {
		m.OOSPF = 99
	}
	m.Passed = trades >= 2 && pnl > 0 && m.OOSPF >= 1.0
	return m
}

// TradeSharpe approximates Sharpe from per-trade USD returns (not annualized).
func TradeSharpe(res *Result) float64 {
	if len(res.Trades) < 3 {
		return 0
	}
	var sum, sumSq float64
	n := float64(len(res.Trades))
	for _, tr := range res.Trades {
		sum += tr.PnLUSD
		sumSq += tr.PnLUSD * tr.PnLUSD
	}
	mean := sum / n
	variance := sumSq/n - mean*mean
	if variance <= 0 {
		return 0
	}
	return mean / math.Sqrt(variance)
}

// DeflatedSharpeRatio adjusts observed Sharpe for multiple-testing (Bailey-López de Prado simplification).
func DeflatedSharpeRatio(sharpe float64, nObs, nTrials int) float64 {
	if nObs < 3 || nTrials < 1 {
		return sharpe
	}
	lnN := math.Log(float64(nTrials))
	if lnN <= 0 {
		return sharpe
	}
	euler := 0.5772156649
	expectedMax := math.Sqrt(2*lnN) * (1 - euler/(2*lnN))
	deflator := expectedMax / math.Sqrt(float64(nObs))
	return sharpe - deflator
}

// SmokeGate is the cheap first filter — delete losers before expensive WFA.
func SmokeGate(res Result) bool {
	if res.TradeCount < 5 {
		return false
	}
	if res.ProfitFactor < 0.90 {
		return false
	}
	if res.MaxDrawdown > 100 {
		return false
	}
	if res.OOSNetPnL <= 0 {
		return false
	}
	return true
}

// HoldoutGate is the strict final segment test before promotion.
func HoldoutGate(m FoldMetrics, minPnL, minPF float64, minTrades int) bool {
	if minPnL <= 0 {
		minPnL = 10
	}
	if minPF <= 0 {
		minPF = 1.05
	}
	if minTrades <= 0 {
		minTrades = 5
	}
	return m.OOSTrades >= minTrades && m.OOSNetPnL >= minPnL && m.OOSPF >= minPF
}

// SessionPnLBetween aggregates PnL by session label in a time window.
func (res *Result) SessionPnLBetween(start, end time.Time) map[string]float64 {
	out := map[string]float64{}
	for _, tr := range res.Trades {
		if tr.EntryTime.Before(start) || !tr.EntryTime.Before(end) {
			continue
		}
		out[tr.Session] += tr.PnLUSD
	}
	return out
}

// TradeFingerprint identifies duplicate trade paths across configs.
func TradeFingerprint(res Result) string {
	if len(res.Trades) == 0 {
		return ""
	}
	cp := append([]Trade(nil), res.Trades...)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].EntryTime.Equal(cp[j].EntryTime) {
			return cp[i].Symbol < cp[j].Symbol
		}
		return cp[i].EntryTime.Before(cp[j].EntryTime)
	})
	var b strings.Builder
	for _, t := range cp {
		b.WriteString(fmt.Sprintf("%s@%s:%s|", t.Symbol, t.EntryTime.Format("2006-01-02T15"), t.Playbook))
	}
	return b.String()
}

// DedupeResults keeps the highest-scoring config per identical trade path.
func DedupeResults(results []Result) []Result {
	best := map[string]Result{}
	for _, res := range results {
		fp := TradeFingerprint(res)
		if fp == "" {
			continue
		}
		if prev, ok := best[fp]; !ok || strategyScore(res, oosProfitFactor(res)) > strategyScore(prev, oosProfitFactor(prev)) {
			best[fp] = res
		}
	}
	out := make([]Result, 0, len(best))
	for _, res := range best {
		out = append(out, res)
	}
	return out
}

func FormatSessionPnL(m map[string]float64) string {
	if len(m) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%.1f", k, m[k]))
	}
	return strings.Join(parts, " ")
}
