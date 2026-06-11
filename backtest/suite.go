package backtest

import (
	"context"
	"fmt"
	"strings"
)

type SuiteReport struct {
	Runs []Result
}

func RunSuite(ctx context.Context, r *Runner, days int, cachedOnly bool) (*SuiteReport, error) {
	if r == nil {
		r = NewRunner(nil)
	}
	cached := cachedOnly
	var runs []RunConfig

	// BT-1: playbook isolation
	for _, pb := range []string{"MOMENTUM_BURST", "SESSION_BREAKOUT", "MEAN_REVERT_VWAP"} {
		runs = append(runs, RunConfig{
			Name: "BT1_" + pb, Days: days, UniverseTopN: 30,
			PlaybooksOnly: []string{pb}, CachedOnly: cached,
		})
	}
	runs = append(runs, RunConfig{Name: "BT1_ALL", Days: days, UniverseTopN: 30, CachedOnly: cached})

	// BT-3: floor variants (on all playbooks)
	for _, f := range []struct {
		name string
		floor int
	}{
		{"BT3_adaptive", 0},
		{"BT3_floor58", 58},
		{"BT3_floor52", 52},
		{"BT3_floor45", 45},
	} {
		runs = append(runs, RunConfig{
			Name: f.name, Days: days, UniverseTopN: 30, FloorOverride: f.floor, CachedOnly: cached,
		})
	}

	// BT-4: exit variants
	runs = append(runs, RunConfig{Name: "BT4_default", Days: days, UniverseTopN: 30, Exit: DefaultExitParams(), CachedOnly: cached})
	runs = append(runs, RunConfig{Name: "BT4_aggressive", Days: days, UniverseTopN: 30, Exit: ExitParams{
		BEAtR: 0.35, PartialAtR: 0.75, PartialPct: 0.5, FullTPAtR: 2.0, StaleHours: 48, TrailATR: 1.2,
	}, CachedOnly: cached})
	runs = append(runs, RunConfig{Name: "BT4_conservative", Days: days, UniverseTopN: 30, Exit: ExitParams{
		BEAtR: 0.75, PartialAtR: 1.5, PartialPct: 0.35, FullTPAtR: 3.0, StaleHours: 96, TrailATR: 2.0,
	}, CachedOnly: cached})

	// BT-5: universe size (skip 200 when using cached subset)
	for _, n := range []int{30, 50} {
		runs = append(runs, RunConfig{
			Name: fmt.Sprintf("BT5_universe%d", n), Days: days, UniverseTopN: n, CachedOnly: cached,
		})
	}

	report := &SuiteReport{}
	for _, cfg := range runs {
		res, err := r.Run(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", cfg.Name, err)
		}
		report.Runs = append(report.Runs, *res)
	}
	return report, nil
}

func FormatSuiteTable(rep *SuiteReport) string {
	var b strings.Builder
	b.WriteString("\n=== AEGIS BACKTEST SUITE (BT-1 → BT-5) ===\n")
	b.WriteString(fmt.Sprintf("%-22s %6s %8s %8s %6s %6s %8s %8s %7s\n",
		"Run", "Trades", "NetPnL", "OOSPnL", "PF", "Win%", "MaxDD", "AvgR", "Flat%"))
	for _, r := range rep.Runs {
		b.WriteString(fmt.Sprintf("%-22s %6d %8.2f %8.2f %6.2f %5.1f%% %8.2f %8.2f %6.1f%%\n",
			r.Config.Name, r.TradeCount, r.NetPnL, r.OOSNetPnL, r.ProfitFactor,
			r.WinRate, r.MaxDrawdown, r.ExpectancyR, r.FlatBarsPct))
	}
	return b.String()
}

func FormatBT2SessionBreakdown(rep *SuiteReport) string {
	var base *Result
	for i := range rep.Runs {
		if rep.Runs[i].Config.Name == "BT1_ALL" {
			base = &rep.Runs[i]
			break
		}
	}
	if base == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n=== BT-2 Session breakdown (BT1_ALL trades) ===\n")
	for sess, n := range base.BySession {
		var pnl float64
		for _, t := range base.Trades {
			if t.Session == sess {
				pnl += t.PnLUSD
			}
		}
		b.WriteString(fmt.Sprintf("  %-10s trades=%3d  pnl=%.2f\n", sess, n, pnl))
	}
	return b.String()
}
