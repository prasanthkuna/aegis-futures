package backtest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// RunSymbolScan runs every playbook book on a single symbol (default BTCUSDT).
func RunSymbolScan(ctx context.Context, r *Runner, symbol string, days int) ([]ScanRow, int, time.Duration, error) {
	if r == nil {
		r = NewRunner(nil)
	}
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	start := time.Now()
	days = r.Store.BestCacheDays(days)
	ds, err := r.LoadDataset(ctx, days, true)
	if err != nil {
		return nil, 0, 0, err
	}
	ds, err = FilterDataset(ds, []string{symbol})
	if err != nil {
		return nil, 0, 0, err
	}
	barCount := len(ds.Timeline)

	def := DefaultExitParams()
	wide := ExitPresets()["wide"]
	pbSets := PlaybookSets()
	type spec struct {
		name string
		cfg  RunConfig
	}
	var specs []spec
	for pbName, pbs := range pbSets {
		specs = append(specs, spec{
			name: pbName + "_adapt_t4",
			cfg: RunConfig{
				Name: pbName, Days: days, UniverseTopN: 1, MaxTradesPerDay: 4,
				PlaybooksOnly: append([]string(nil), pbs...),
				Exit: def, CachedOnly: true,
			},
		})
	}
	// Knob variants on the multi-symbol winner book.
	revertPB := []string{"SESSION_BREAKOUT", "MEAN_REVERT_VWAP"}
	for _, v := range []struct {
		name string
		cfg  RunConfig
	}{
		{"SESSION+REVERT_f58_t4", RunConfig{Days: days, UniverseTopN: 1, MaxTradesPerDay: 4,
			PlaybooksOnly: revertPB, FloorOverride: 58, Exit: def, CachedOnly: true}},
		{"SESSION+REVERT_wide_exit", RunConfig{Days: days, UniverseTopN: 1, MaxTradesPerDay: 4,
			PlaybooksOnly: revertPB, Exit: wide, CachedOnly: true}},
		{"SESSION+REVERT_asia_only", RunConfig{Days: days, UniverseTopN: 1, MaxTradesPerDay: 4,
			PlaybooksOnly: revertPB, SessionsOnly: []string{"asia"}, Exit: def, CachedOnly: true}},
		{"SESSION+REVERT_f58_asia", RunConfig{Days: days, UniverseTopN: 1, MaxTradesPerDay: 4,
			PlaybooksOnly: revertPB, FloorOverride: 58, SessionsOnly: []string{"asia"}, Exit: def, CachedOnly: true}},
	} {
		v.cfg.Name = v.name
		specs = append(specs, spec{name: v.name, cfg: v.cfg})
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].name < specs[j].name })

	fmt.Printf("Symbol scan: %s | %d strategies | %dd | %d bars\n",
		symbol, len(specs), days, len(ds.Timeline))

	var rows []ScanRow
	for i, sp := range specs {
		sp.cfg.Name = sp.name
		res, err := r.RunDataset(ds, sp.cfg)
		if err != nil {
			return nil, 0, 0, err
		}
		topPB, topPBPnL := topKeyPnL(res.Trades, true)
		topSess, topSessPnL := topKeyPnL(res.Trades, false)
		oosPF := oosProfitFactor(*res)
		grade := scanGrade(*res, oosPF)
		rows = append(rows, ScanRow{
			Name: sp.name, NetPnL: res.NetPnL, OOSPnL: res.OOSNetPnL, OOSPF: oosPF,
			PF: res.ProfitFactor, MaxDD: res.MaxDrawdown, Trades: res.TradeCount,
			WinRate: res.WinRate, TopSymbol: topPB, TopSymPnL: topPBPnL,
			TopSession: topSess, TopSessPnL: topSessPnL, Grade: grade,
		})
		fmt.Printf("  [%d/%d] %s net=%.2f oos=%.2f tr=%d [%s]\n",
			i+1, len(specs), sp.name, res.NetPnL, res.OOSNetPnL, res.TradeCount, grade)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].NetPnL != rows[j].NetPnL {
			return rows[i].NetPnL > rows[j].NetPnL
		}
		return rows[i].OOSPnL > rows[j].OOSPnL
	})
	return rows, barCount, time.Since(start), nil
}

func FormatSymbolScan(symbol string, rows []ScanRow, days int, bars int, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n=== %s STRATEGY SCAN (all playbooks, single symbol) ===\n", symbol))
	b.WriteString(fmt.Sprintf("%dd | %d bars | %s\n\n", days, bars, elapsed.Round(time.Second)))

	winA, winB := 0, 0
	for _, r := range rows {
		switch r.Grade {
		case "A":
			winA++
		case "B":
			winB++
		}
	}
	b.WriteString(fmt.Sprintf("Winners: Grade A=%d  B=%d  total tested=%d\n\n", winA, winB, len(rows)))
	b.WriteString(fmt.Sprintf("%-4s %-30s %8s %8s %5s %5s %4s %s\n",
		"Grd", "Strategy", "Net", "OOS", "OOSPF", "DD", "Tr", "Top playbook"))
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("%-4s %-30s %8.2f %8.2f %5.2f %5.0f %4d %s $%.1f\n",
			r.Grade, trunc(r.Name, 30), r.NetPnL, r.OOSPnL, r.OOSPF, r.MaxDD, r.Trades, r.TopSymbol, r.TopSymPnL))
	}

	b.WriteString("\n=== PROMOTE LIST (Grade A/B on " + symbol + ") ===\n")
	n := 0
	for _, r := range rows {
		if r.Grade != "A" && r.Grade != "B" {
			continue
		}
		n++
		b.WriteString(fmt.Sprintf("%d. [%s] %s — net $%.2f | OOS $%.2f | %d trades | WR %.0f%%\n",
			n, r.Grade, r.Name, r.NetPnL, r.OOSPnL, r.Trades, r.WinRate))
	}
	if n == 0 {
		b.WriteString("  No net+OOS winners on this symbol in this window.\n")
		b.WriteString("  Best by OOS:\n")
		sort.Slice(rows, func(i, j int) bool { return rows[i].OOSPnL > rows[j].OOSPnL })
		for i := 0; i < 3 && i < len(rows); i++ {
			r := rows[i]
			b.WriteString(fmt.Sprintf("  - %s OOS $%.2f net $%.2f\n", r.Name, r.OOSPnL, r.NetPnL))
		}
	}
	return b.String()
}
