package backtest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"encore.app/config"
)

// CoreProvenRow is one symbol × horizon with fixed prod params.
type CoreProvenRow struct {
	Symbol     string
	Strategy   string
	Days       int
	Bars1h     int
	RiskUSD    float64
	Leverage   int
	MaxPerDay  int
	RR         float64
	StopATR    float64
	NetPnL     float64
	OOSPnL     float64
	PF         float64
	MaxDD      float64
	Trades     int
	WinRate    float64
	Profitable bool
	ROIOn200   float64 // scaled: net * (200/250) * (4/5) ≈ net * 0.64 for $200 @ 2%
}

type coreProvenSpec struct {
	symbol   string
	strategy string
	fn       BTCStrategyFunc
	par      btcLabParams
}

func coreProvenSpecs() []coreProvenSpec {
	return []coreProvenSpec{
		{
			symbol: "BTCUSDT", strategy: "S4_SQUEEZE_LIBERAL",
			fn: S4_SQUEEZE_LIBERAL,
			par: btcLabParams{stopATR: 1.4, rr: 4.0, maxHold: 36, cooldown: 2, maxPerDay: 4},
		},
		{
			symbol: "SOLUSDT", strategy: "S11_EMA_TREND_LIBERAL",
			fn: S11_EMA_TREND_LIBERAL,
			par: btcLabParams{stopATR: 1.2, rr: 3.0, maxHold: 36, cooldown: 2, maxPerDay: 4},
		},
		{
			symbol: "ETHUSDT", strategy: "S11_EMA_TREND_STRICT",
			fn: S11_H1_EMA_TREND,
			par: btcLabParams{stopATR: 1.8, rr: 4.0, maxHold: 36, cooldown: 2, maxPerDay: 4},
		},
	}
}

// solS4ProvenSpec — SOL alt from 60d prod (S4 liberal beat S11 on shorter window).
func solS4ProvenSpec() coreProvenSpec {
	return coreProvenSpec{
		symbol: "SOLUSDT", strategy: "S4_SQUEEZE_LIBERAL",
		fn: S4_SQUEEZE_LIBERAL,
		par: btcLabParams{stopATR: 1.8, rr: 4.0, maxHold: 36, cooldown: 2, maxPerDay: 4},
	}
}

// RunCoreProven validates prod strategies on requested history (fetches if missing).
func RunCoreProven(ctx context.Context, r *Runner, days int) ([]CoreProvenRow, time.Duration, error) {
	return runCoreProvenSpecs(ctx, r, days, coreProvenSpecs())
}

// RunSOLS4Proven validates SOL S4 squeeze liberal on 90/180d horizons.
func RunSOLS4Proven(ctx context.Context, r *Runner, days int) ([]CoreProvenRow, time.Duration, error) {
	return runCoreProvenSpecs(ctx, r, days, []coreProvenSpec{solS4ProvenSpec()})
}

func runCoreProvenSpecs(ctx context.Context, r *Runner, days int, specs []coreProvenSpec) ([]CoreProvenRow, time.Duration, error) {
	if r == nil {
		r = NewRunner(nil)
	}
	start := time.Now()
	if days <= 0 {
		days = 90
	}
	sz := btcSizing{
		riskUSD:       config.BTCProdRiskUSD,
		activeCapital: config.BTCProdActiveCapital,
		leverage:      config.BTCProdMaxLeverage,
	}
	var rows []CoreProvenRow
	for _, spec := range specs {
		bars5m, err := r.Store.LoadOrFetch(ctx, spec.symbol, days)
		if err != nil {
			return nil, 0, fmt.Errorf("%s %dd: %w", spec.symbol, days, err)
		}
		bars1h := resampleBars(bars5m, config.BTCProdTimeframeBars)
		if len(bars1h) < 80 {
			return nil, 0, fmt.Errorf("%s %dd: only %d 1h bars", spec.symbol, days, len(bars1h))
		}
		oosIdx := int(float64(len(bars1h)) * 0.7)
		res := runOneBTCStrategySized(spec.strategy, spec.fn, bars1h, oosIdx, spec.par, sz)
		scale := (200.0 / sz.activeCapital) * (4.0 / sz.riskUSD) // $200 @ $4 risk vs $250 @ $5
		row := CoreProvenRow{
			Symbol: spec.symbol, Strategy: spec.strategy, Days: days,
			Bars1h: len(bars1h),
			RiskUSD: sz.riskUSD, Leverage: sz.leverage, MaxPerDay: spec.par.maxPerDay,
			RR: spec.par.rr, StopATR: spec.par.stopATR,
			NetPnL: res.NetPnL, OOSPnL: res.OOSPnL, PF: res.PF,
			MaxDD: res.MaxDD, Trades: res.Trades, WinRate: res.WinRate,
			Profitable: res.Profitable,
			ROIOn200: res.NetPnL * scale / 200 * 100,
		}
		rows = append(rows, row)
		fmt.Printf("  %s %dd %s net=%.2f oos=%.2f tr=%d %s\n",
			spec.symbol, days, spec.strategy, row.NetPnL, row.OOSPnL, row.Trades,
			map[bool]string{true: "WIN", false: "FAIL"}[row.Profitable])
	}
	return rows, time.Since(start), nil
}

func FormatCoreProven(rows []CoreProvenRow, elapsed time.Duration) string {
	var b strings.Builder
	if len(rows) == 0 {
		return "No results.\n"
	}
	days := rows[0].Days
	b.WriteString(fmt.Sprintf("\n=== CORE PROVEN — fixed prod params | %dd | %s ===\n", days, elapsed.Round(time.Second)))
	b.WriteString("$5 risk | 5x | max 4/day | 1h | net+OOS must both be green\n\n")
	b.WriteString("BTC → S4_SQUEEZE_LIBERAL rr4.0 stop1.4\n")
	b.WriteString("SOL → S11_EMA_TREND_LIBERAL rr3.0 stop1.2\n")
	b.WriteString("ETH → S11_EMA_TREND_STRICT rr4.0 stop1.8\n\n")
	b.WriteString(fmt.Sprintf("%-10s %-24s %7s %7s %5s %5s %6s %4s %s\n",
		"Symbol", "Strategy", "Net", "OOS", "PF", "DD", "ROI200", "Tr", "OK"))
	for _, r := range rows {
		ok := "FAIL"
		if r.Profitable {
			ok = "WIN"
		}
		b.WriteString(fmt.Sprintf("%-10s %-24s %7.2f %7.2f %5.2f %5.0f %5.1f%% %4d %s\n",
			r.Symbol, r.Strategy, r.NetPnL, r.OOSPnL, r.PF, r.MaxDD, r.ROIOn200, r.Trades, ok))
	}
	win := 0
	for _, r := range rows {
		if r.Profitable {
			win++
		}
	}
	b.WriteString(fmt.Sprintf("\nProfitable: %d / %d\n", win, len(rows)))
	return b.String()
}

func FormatSOLS4Proven(rows []CoreProvenRow, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString("\n=== SOL S4 SQUEEZE LIBERAL — 90d & 180d ===\n")
	b.WriteString(fmt.Sprintf("Runtime: %s | rr4.0 stop1.8 | $5 risk 5x max4/day\n", elapsed.Round(time.Second)))
	byDays := map[int][]CoreProvenRow{}
	for _, r := range rows {
		byDays[r.Days] = append(byDays[r.Days], r)
	}
	keys := make([]int, 0, len(byDays))
	for d := range byDays {
		keys = append(keys, d)
	}
	sort.Ints(keys)
	for _, d := range keys {
		for _, r := range byDays[d] {
			ok := "FAIL"
			if r.Profitable {
				ok = "WIN"
			}
			b.WriteString(fmt.Sprintf("%dd: net $%.2f | OOS $%.2f | PF %.2f | DD $%.0f | %d tr | ROI200 %.1f%% [%s]\n",
				d, r.NetPnL, r.OOSPnL, r.PF, r.MaxDD, r.Trades, r.ROIOn200, ok))
		}
	}
	b.WriteString("\nCompare SOL S11 liberal 90d: net -$39.62 | 180d: net -$152.40 (both FAIL)\n")
	return b.String()
}

func FormatCoreProvenMulti(byDays map[int][]CoreProvenRow, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString("\n=== CORE PROVEN — 90d & 180d validation ===\n")
	b.WriteString(fmt.Sprintf("Total runtime: %s\n", elapsed.Round(time.Second)))
	keys := make([]int, 0, len(byDays))
	for d := range byDays {
		keys = append(keys, d)
	}
	sort.Ints(keys)
	for _, d := range keys {
		b.WriteString(FormatCoreProven(byDays[d], 0))
	}
	return b.String()
}
