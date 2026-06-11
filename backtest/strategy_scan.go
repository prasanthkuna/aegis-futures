package backtest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ScanRow is one strategy candidate on u30/60d.
type ScanRow struct {
	Name       string
	NetPnL     float64
	OOSPnL     float64
	OOSPF      float64
	PF         float64
	MaxDD      float64
	Trades     int
	WinRate    float64
	TopSymbol  string
	TopSymPnL  float64
	TopSession string
	TopSessPnL float64
	Grade      string
}

// RunStrategyScan tests high-value hypotheses (not factorial grid).
func RunStrategyScan(ctx context.Context, r *Runner, days int) ([]ScanRow, time.Duration, error) {
	if r == nil {
		r = NewRunner(nil)
	}
	start := time.Now()
	days = r.Store.BestCacheDays(days)
	ds, err := r.LoadDataset(ctx, days, true)
	if err != nil {
		return nil, 0, err
	}
	def := DefaultExitParams()
	cons := ExitPresets()["conservative"]
	wide := ExitPresets()["wide"]
	revertPB := []string{"SESSION_BREAKOUT", "MEAN_REVERT_VWAP"}

	type spec struct {
		name string
		cfg  RunConfig
	}
	specs := []spec{
		{"BASELINE_adapt_all_t4", RunConfig{Name: "b", Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: revertPB, Exit: def, CachedOnly: true}},
		{"asia_only_adapt_t4", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: revertPB, SessionsOnly: []string{"asia"}, Exit: def, CachedOnly: true}},
		{"london_us_only_adapt_t4", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: revertPB, SessionsOnly: []string{"london", "us"}, Exit: def, CachedOnly: true}},
		{"floor52_all_t4", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: revertPB, FloorOverride: 52, Exit: def, CachedOnly: true}},
		{"floor58_all_t4", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: revertPB, FloorOverride: 58, Exit: def, CachedOnly: true}},
		{"adapt_t2_cap", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 2,
			PlaybooksOnly: revertPB, Exit: def, CachedOnly: true}},
		{"adapt_t6_cap", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 6,
			PlaybooksOnly: revertPB, Exit: def, CachedOnly: true}},
		{"adapt_conservative_exit", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: revertPB, Exit: cons, CachedOnly: true}},
		{"adapt_wide_exit", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: revertPB, Exit: wide, CachedOnly: true}},
		{"REVERT_solo_adapt", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: []string{"MEAN_REVERT_VWAP"}, Exit: def, CachedOnly: true}},
		{"SESSION_solo_adapt", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: []string{"SESSION_BREAKOUT"}, Exit: def, CachedOnly: true}},
		{"VOL_CLIMAX_solo", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: []string{"VOL_CLIMAX_FADE"}, Exit: def, CachedOnly: true}},
		{"REVERT+VOL_CLIMAX", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: []string{"MEAN_REVERT_VWAP", "VOL_CLIMAX_FADE"}, Exit: def, CachedOnly: true}},
		{"SESSION+REVERT+VOL", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: []string{"SESSION_BREAKOUT", "MEAN_REVERT_VWAP", "VOL_CLIMAX_FADE"}, Exit: def, CachedOnly: true}},
		{"FORCED_FLOW_solo", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: []string{"FORCED_FLOW_FADE"}, Exit: def, CachedOnly: true}},
		{"ALL_default_playbooks", RunConfig{Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			Exit: def, CachedOnly: true}},
		{"u50_BASELINE", RunConfig{Days: days, UniverseTopN: 50, MaxTradesPerDay: 4,
			PlaybooksOnly: revertPB, Exit: def, CachedOnly: true}},
	}
	for i := range specs {
		specs[i].cfg.Name = specs[i].name
	}

	fmt.Printf("Strategy scan: %d configs | %dd | u30 (1x u50) | %d symbols\n",
		len(specs), days, len(ds.All))

	var rows []ScanRow
	for i, sp := range specs {
		res, err := r.RunDataset(ds, sp.cfg)
		if err != nil {
			return nil, 0, err
		}
		topSym, topSymPnL := topTradeKey(res.Trades, func(t Trade) string { return t.Symbol })
		topSess, topSessPnL := topKeyPnL(res.Trades, false)
		oosPF := oosProfitFactor(*res)
		grade := scanGrade(*res, oosPF)
		rows = append(rows, ScanRow{
			Name: sp.name, NetPnL: res.NetPnL, OOSPnL: res.OOSNetPnL, OOSPF: oosPF,
			PF: res.ProfitFactor, MaxDD: res.MaxDrawdown, Trades: res.TradeCount,
			WinRate: res.WinRate, TopSymbol: topSym, TopSymPnL: topSymPnL,
			TopSession: topSess, TopSessPnL: topSessPnL, Grade: grade,
		})
		fmt.Printf("  [%d/%d] %s net=%.1f oos=%.1f [%s]\n",
			i+1, len(specs), sp.name, res.NetPnL, res.OOSNetPnL, grade)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].NetPnL != rows[j].NetPnL {
			return rows[i].NetPnL > rows[j].NetPnL
		}
		return rows[i].OOSPnL > rows[j].OOSPnL
	})
	return rows, time.Since(start), nil
}

func topTradeKey(trades []Trade, key func(Trade) string) (string, float64) {
	m := map[string]float64{}
	for _, t := range trades {
		m[key(t)] += t.PnLUSD
	}
	bestK, bestV := "", 0.0
	for k, v := range m {
		if v > bestV {
			bestK, bestV = k, v
		}
	}
	return bestK, bestV
}

func scanGrade(res Result, oosPF float64) string {
	if res.NetPnL > 0 && res.OOSNetPnL > 0 && oosPF >= 1.05 {
		return "A"
	}
	if res.NetPnL > 0 && res.OOSNetPnL > 0 {
		return "B"
	}
	if res.OOSNetPnL > 0 {
		return "C"
	}
	return "F"
}

func FormatStrategyScan(rows []ScanRow, days int, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString("\n=== STRATEGY SCAN (profitable candidates on u30/60d) ===\n")
	b.WriteString(fmt.Sprintf("%dd | %s | sorted by net PnL\n\n", days, elapsed.Round(time.Second)))

	winners := 0
	for _, r := range rows {
		if r.Grade == "A" || r.Grade == "B" {
			winners++
		}
	}
	b.WriteString(fmt.Sprintf("Grade A/B (net+OOS green): %d / %d\n\n", winners, len(rows)))
	b.WriteString(fmt.Sprintf("%-4s %-28s %8s %8s %5s %6s %5s %s\n",
		"Grd", "Config", "Net", "OOS", "OOSPF", "DD", "Tr", "Top edge"))
	for _, r := range rows {
		edge := r.TopSession
		if r.TopSessPnL != 0 {
			edge = fmt.Sprintf("%s $%.0f", r.TopSession, r.TopSessPnL)
		}
		b.WriteString(fmt.Sprintf("%-4s %-28s %8.1f %8.1f %5.2f %6.0f %5d %s\n",
			r.Grade, trunc(r.Name, 28), r.NetPnL, r.OOSPnL, r.OOSPF, r.MaxDD, r.Trades, edge))
	}

	b.WriteString("\n=== GRADE A/B DETAIL ===\n")
	n := 0
	for _, r := range rows {
		if r.Grade != "A" && r.Grade != "B" {
			continue
		}
		n++
		b.WriteString(fmt.Sprintf("\n#%d [%s] %s\n", n, r.Grade, r.Name))
		b.WriteString(fmt.Sprintf("  Net $%.1f | OOS $%.1f | PF %.2f | OOS PF %.2f | DD $%.0f | WR %.0f%% | %d trades\n",
			r.NetPnL, r.OOSPnL, r.PF, r.OOSPF, r.MaxDD, r.WinRate, r.Trades))
		b.WriteString(fmt.Sprintf("  Top symbol: %s ($%.1f) | Top session: %s ($%.1f)\n",
			r.TopSymbol, r.TopSymPnL, r.TopSession, r.TopSessPnL))
	}
	if n == 0 {
		b.WriteString("  None — tighten hypotheses or extend data.\n")
	}

	return b.String()
}

// RunStrategyScanWithAttribution runs scan and appends baseline symbol breakdown.
func RunStrategyScanWithAttribution(ctx context.Context, r *Runner, days int) (string, time.Duration, error) {
	rows, elapsed, err := RunStrategyScan(ctx, r, days)
	if err != nil {
		return "", 0, err
	}
	days = r.Store.BestCacheDays(days)
	ds, err := r.LoadDataset(ctx, days, true)
	if err != nil {
		return "", 0, err
	}
	base, err := r.RunDataset(ds, RunConfig{
		Name: "BASELINE", Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
		PlaybooksOnly: []string{"SESSION_BREAKOUT", "MEAN_REVERT_VWAP"},
		Exit: DefaultExitParams(), CachedOnly: true,
	})
	if err != nil {
		return "", 0, err
	}
	var b strings.Builder
	b.WriteString(FormatStrategyScan(rows, days, elapsed))
	AppendSymbolAttribution(&b, *base)
	return b.String(), elapsed, nil
}

// SymbolAttribution returns top symbols by PnL for a result.
func SymbolAttribution(res Result, topN int) []struct {
	Symbol string
	PnL    float64
	Trades int
} {
	m := map[string]struct{ pnl float64; n int }{}
	for _, t := range res.Trades {
		x := m[t.Symbol]
		x.pnl += t.PnLUSD
		x.n++
		m[t.Symbol] = x
	}
	type row struct {
		Symbol string
		PnL    float64
		Trades int
	}
	var rows []row
	for sym, v := range m {
		rows = append(rows, row{sym, v.pnl, v.n})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].PnL > rows[j].PnL })
	if topN <= 0 || topN > len(rows) {
		topN = len(rows)
	}
	out := make([]struct {
		Symbol string
		PnL    float64
		Trades int
	}, topN)
	for i := 0; i < topN; i++ {
		out[i].Symbol = rows[i].Symbol
		out[i].PnL = rows[i].PnL
		out[i].Trades = rows[i].Trades
	}
	return out
}

func AppendSymbolAttribution(b *strings.Builder, res Result) {
	b.WriteString("\n=== TOP SYMBOLS (baseline) ===\n")
	for i, r := range SymbolAttribution(res, 15) {
		b.WriteString(fmt.Sprintf("  %2d. %-16s $%7.1f  (%d trades)\n", i+1, r.Symbol, r.PnL, r.Trades))
	}
}
