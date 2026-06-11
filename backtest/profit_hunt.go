package backtest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ProfitHuntRow is one profitable config ranked by net PnL on full sample.
type ProfitHuntRow struct {
	Rank          int
	Name          string
	Playbooks     string
	Floor         string
	Universe      int
	MaxTrades     int
	Exit          string
	Sessions      string
	NetPnL        float64
	OOSPnL        float64
	PF            float64
	OOSPF         float64
	MaxDD         float64
	Trades        int
	WinRate       float64
	UniquePath    bool
	FingerprintID int
}

// ProfitHuntReport lists the most profitable configs on cached data.
type ProfitHuntReport struct {
	Days           int
	Symbols        int
	Bars           int
	TotalRuns      int
	Profitable     int
	UniquePaths    int
	TopByNet       []ProfitHuntRow
	TopUniquePaths []ProfitHuntRow
	Elapsed        time.Duration
}

// GenerateProfitHuntGrid sweeps playbooks × floors × universe × trade cap × exits (all sessions).
func GenerateProfitHuntGrid(days int) []RunConfig {
	if days <= 0 {
		days = 60
	}
	return generateGrid(gridSpec{
		days:      days,
		floors:    []floorOpt{{"adapt", 0}, {"f55", 55}, {"f52", 52}, {"f58", 58}},
		universes: []int{30, 50},
		maxTrades: []int{2, 4, 6},
		exitNames: []string{"default", "conservative", "wide", "tight", "runner"},
		sessions:  []sessOpt{{label: "all", val: nil}},
	})
}

// RunProfitHunt scans the hunt grid and ranks by net PnL — no WFA/holdout gates.
func RunProfitHunt(ctx context.Context, r *Runner, days int) (*ProfitHuntReport, error) {
	if r == nil {
		r = NewRunner(nil)
	}
	start := time.Now()
	days = r.Store.BestCacheDays(days)
	ds, err := r.LoadDataset(ctx, days, true)
	if err != nil {
		return nil, err
	}

	grid := GenerateProfitHuntGrid(days)
	fmt.Printf("Profit hunt: %d configs | %dd | %d symbols | rank by net PnL\n",
		len(grid), days, len(ds.All))

	var results []Result
	for i, cfg := range grid {
		res, err := r.RunDataset(ds, cfg)
		if err != nil {
			continue
		}
		results = append(results, *res)
		if (i+1)%60 == 0 || i+1 == len(grid) {
			fmt.Printf("  scanned %d/%d\n", i+1, len(grid))
		}
	}

	rep := &ProfitHuntReport{
		Days: days, Symbols: len(ds.All), Bars: len(ds.Timeline), TotalRuns: len(results),
	}

	// All configs with net > 0, sorted by net PnL.
	var profitable []Result
	for _, res := range results {
		if res.NetPnL > 0 {
			profitable = append(profitable, res)
			rep.Profitable++
		}
	}
	sort.Slice(profitable, func(i, j int) bool {
		if profitable[i].NetPnL != profitable[j].NetPnL {
			return profitable[i].NetPnL > profitable[j].NetPnL
		}
		return profitable[i].OOSNetPnL > profitable[j].OOSNetPnL
	})

	fpID := map[string]int{}
	nextID := 1
	for _, res := range profitable {
		fp := TradeFingerprint(res)
		if fp != "" {
			if _, ok := fpID[fp]; !ok {
				fpID[fp] = nextID
				nextID++
			}
		}
	}
	rep.UniquePaths = len(fpID)

	for i, res := range profitable {
		row := resultToHuntRow(res, i+1, fpID)
		rep.TopByNet = append(rep.TopByNet, row)
	}

	// Best net PnL per unique trade path.
	bestPerFP := map[string]Result{}
	for _, res := range profitable {
		fp := TradeFingerprint(res)
		if fp == "" {
			continue
		}
		if prev, ok := bestPerFP[fp]; !ok || res.NetPnL > prev.NetPnL {
			bestPerFP[fp] = res
		}
	}
	var unique []Result
	for _, res := range bestPerFP {
		unique = append(unique, res)
	}
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].NetPnL > unique[j].NetPnL
	})
	for i, res := range unique {
		row := resultToHuntRow(res, i+1, fpID)
		row.UniquePath = true
		rep.TopUniquePaths = append(rep.TopUniquePaths, row)
	}

	rep.Elapsed = time.Since(start)
	return rep, nil
}

func resultToHuntRow(res Result, rank int, fpID map[string]int) ProfitHuntRow {
	cfg := res.Config
	exitName := "default"
	for name, p := range ExitPresets() {
		if p == cfg.Exit {
			exitName = name
			break
		}
	}
	fp := TradeFingerprint(res)
	id := 0
	if fp != "" {
		id = fpID[fp]
	}
	return ProfitHuntRow{
		Rank: rank, Name: cfg.Name,
		Playbooks: pbLabel(cfg.PlaybooksOnly), Floor: floorLabel(cfg.FloorOverride),
		Universe: cfg.UniverseTopN, MaxTrades: cfg.maxTradesDay(), Exit: exitName,
		Sessions: sessLabel(cfg.SessionsOnly),
		NetPnL: res.NetPnL, OOSPnL: res.OOSNetPnL, PF: res.ProfitFactor,
		OOSPF: oosProfitFactor(res), MaxDD: res.MaxDrawdown, Trades: res.TradeCount,
		WinRate: res.WinRate, FingerprintID: id,
	}
}

func FormatProfitHuntReport(rep *ProfitHuntReport) string {
	if rep == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n=== PROFIT HUNT (ranked by NET PnL on full sample) ===\n")
	b.WriteString(fmt.Sprintf("Data: %dd | %d symbols | %d bars | %d configs | %d profitable | %d unique trade paths | %s\n\n",
		rep.Days, rep.Symbols, rep.Bars, rep.TotalRuns, rep.Profitable, rep.UniquePaths, rep.Elapsed.Round(time.Second)))

	if rep.Profitable == 0 {
		b.WriteString("No net-profitable configs in hunt grid.\n")
		return b.String()
	}

	b.WriteString("--- TOP 15 BY NET PnL (includes same trades, different exits/labels) ---\n")
	b.WriteString(fmt.Sprintf("%-3s %-44s %8s %8s %5s %6s %5s %4s path#%s\n",
		"#", "Strategy", "Net", "OOS", "PF", "OOSPF", "DD", "Tr", ""))
	limit := 15
	if len(rep.TopByNet) < limit {
		limit = len(rep.TopByNet)
	}
	for i := 0; i < limit; i++ {
		r := rep.TopByNet[i]
		b.WriteString(fmt.Sprintf("%-3d %-44s %8.1f %8.1f %5.2f %6.2f %5.0f %5d %d\n",
			r.Rank, trunc(r.Name, 44), r.NetPnL, r.OOSPnL, r.PF, r.OOSPF, r.MaxDD, r.Trades, r.FingerprintID))
	}

	b.WriteString("\n--- TOP UNIQUE TRADE PATHS (deduped — true distinct strategies) ---\n")
	b.WriteString(fmt.Sprintf("%-3s %-44s %8s %8s %5s %s\n", "#", "Strategy", "Net", "OOS", "PF", "Knobs"))
	ulimit := 10
	if len(rep.TopUniquePaths) < ulimit {
		ulimit = len(rep.TopUniquePaths)
	}
	for i := 0; i < ulimit; i++ {
		r := rep.TopUniquePaths[i]
		knobs := fmt.Sprintf("%s | %s floor | u%d | t%d | %s exit", r.Playbooks, r.Floor, r.Universe, r.MaxTrades, r.Exit)
		b.WriteString(fmt.Sprintf("%-3d %-44s %8.1f %8.1f %5.2f %s\n",
			i+1, trunc(r.Name, 44), r.NetPnL, r.OOSPnL, r.PF, knobs))
	}

	b.WriteString("\n--- TOP 5 FOR YOUR DESK (highest net, diverse playbooks where possible) ---\n")
	picked := pickTop5Diverse(rep)
	for i, r := range picked {
		b.WriteString(fmt.Sprintf("\n#%d %s\n", i+1, r.Name))
		b.WriteString(fmt.Sprintf("  Net $%.1f | OOS $%.1f | PF %.2f | OOS PF %.2f | DD $%.0f | %d trades | WR %.0f%%\n",
			r.NetPnL, r.OOSPnL, r.PF, r.OOSPF, r.MaxDD, r.Trades, r.WinRate))
		b.WriteString(fmt.Sprintf("  %s | %s floor | universe top %d | max %d trades/day | %s exit | sessions: %s\n",
			r.Playbooks, r.Floor, r.Universe, r.MaxTrades, r.Exit, r.Sessions))
		if r.FingerprintID > 0 {
			b.WriteString(fmt.Sprintf("  Trade path id: %d (configs with same id = identical entries)\n", r.FingerprintID))
		}
	}
	return b.String()
}

func pickTop5Diverse(rep *ProfitHuntReport) []ProfitHuntRow {
	if len(rep.TopByNet) == 0 {
		return nil
	}
	var out []ProfitHuntRow
	seenFP := map[int]bool{}
	seenPB := map[string]bool{}

	// First pass: best net per unique fingerprint.
	for _, r := range rep.TopByNet {
		if r.FingerprintID > 0 && seenFP[r.FingerprintID] {
			continue
		}
		if r.FingerprintID > 0 {
			seenFP[r.FingerprintID] = true
		}
		seenPB[r.Playbooks] = true
		out = append(out, r)
		if len(out) >= 5 {
			return out
		}
	}
	// Fill from unique paths if needed.
	for _, r := range rep.TopUniquePaths {
		if len(out) >= 5 {
			break
		}
		if r.FingerprintID > 0 && seenFP[r.FingerprintID] {
			continue
		}
		dup := false
		for _, p := range out {
			if p.Name == r.Name {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		if r.FingerprintID > 0 {
			seenFP[r.FingerprintID] = true
		}
		out = append(out, r)
	}
	return out
}
