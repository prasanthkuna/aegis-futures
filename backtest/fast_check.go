package backtest

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// FastCheckRow is one quick backtest result.
type FastCheckRow struct {
	Name    string
	NetPnL  float64
	OOSPnL  float64
	OOSPF   float64
	Trades  int
	Verdict string
}

// RunFastCheck compares baseline vs repo-import playbooks on u30 / 60d.
func RunFastCheck(ctx context.Context, r *Runner, days int) ([]FastCheckRow, time.Duration, error) {
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
	cfgs := []RunConfig{
		{Name: "BASELINE_SESSION+REVERT", Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: []string{"SESSION_BREAKOUT", "MEAN_REVERT_VWAP"}, Exit: def, CachedOnly: true},
		{Name: "BB_SOLO", Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: []string{"BB_STRETCH_REVERT"}, Exit: def, CachedOnly: true},
		{Name: "REVERT+BB", Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: []string{"MEAN_REVERT_VWAP", "BB_STRETCH_REVERT"}, Exit: def, CachedOnly: true},
		{Name: "SESSION+REVERT+BB", Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			PlaybooksOnly: []string{"SESSION_BREAKOUT", "MEAN_REVERT_VWAP", "BB_STRETCH_REVERT"}, Exit: def, CachedOnly: true},
		{Name: "ALL_PLAYBOOKS", Days: days, UniverseTopN: 30, MaxTradesPerDay: 4,
			Exit: def, CachedOnly: true},
	}
	fmt.Printf("Fast check: %d configs | %dd | u30 | %d symbols\n", len(cfgs), days, len(ds.All))

	var rows []FastCheckRow
	for i, cfg := range cfgs {
		res, err := r.RunDataset(ds, cfg)
		if err != nil {
			return nil, 0, err
		}
		oosPF := oosProfitFactor(*res)
		v := "FAIL"
		if res.NetPnL > 0 && res.OOSNetPnL > 0 {
			v = "PASS"
		} else if res.OOSNetPnL > 0 {
			v = "OOS_ONLY"
		}
		rows = append(rows, FastCheckRow{
			Name: cfg.Name, NetPnL: res.NetPnL, OOSPnL: res.OOSNetPnL,
			OOSPF: oosPF, Trades: res.TradeCount, Verdict: v,
		})
		fmt.Printf("  [%d/%d] %s net=%.1f oos=%.1f trades=%d\n",
			i+1, len(cfgs), cfg.Name, res.NetPnL, res.OOSNetPnL, res.TradeCount)
	}
	return rows, time.Since(start), nil
}

func FormatFastCheck(rows []FastCheckRow, days int, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString("\n=== FAST CHECK (repo import vs baseline) ===\n")
	b.WriteString(fmt.Sprintf("u30 | %dd | %s\n\n", days, elapsed.Round(time.Second)))
	b.WriteString(fmt.Sprintf("%-28s %8s %8s %6s %6s %s\n", "Config", "Net", "OOS", "OOSPF", "Trades", "Verdict"))
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("%-28s %8.1f %8.1f %6.2f %6d %s\n",
			r.Name, r.NetPnL, r.OOSPnL, r.OOSPF, r.Trades, r.Verdict))
	}
	b.WriteString("\nPASS = net+OOS green | OOS_ONLY = OOS green but net negative | FAIL = no OOS edge\n")
	return b.String()
}
