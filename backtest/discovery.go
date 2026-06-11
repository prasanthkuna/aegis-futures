package backtest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// CatalogTier classifies historical profitability for future application (not live promotion).
type CatalogTier string

const (
	TierA CatalogTier = "A" // strong OOS: apply in similar regime, paper first
	TierB CatalogTier = "B" // OOS green: watchlist
	TierC CatalogTier = "C" // IS only: likely overfit — archive only
	TierD CatalogTier = "D" // unprofitable
)

// CatalogEntry is one backtested config with regime context for future use.
type CatalogEntry struct {
	Tier           CatalogTier
	Config         RunConfig
	NetPnL         float64
	OOSPnL         float64
	OOSPF          float64
	PF             float64
	MaxDD          float64
	Trades         int
	OOSTrades      int
	WinRate        float64
	WFAFoldPnL     []float64
	WFAPasses      int
	HoldoutPnL     float64
	HoldoutPF      float64
	BestSession    string
	ApplyWhen      string
	Caution        string
}

// DiscoveryReport is the research catalog output.
type DiscoveryReport struct {
	Days       int
	Symbols    int
	Bars       int
	TotalRuns  int
	Profitable int
	TierA      int
	TierB      int
	TierC      int
	Entries    []CatalogEntry
	Elapsed    time.Duration
}

// RunDiscovery scans configs and catalogs every historically profitable strategy.
func RunDiscovery(ctx context.Context, r *Runner, days int, mode string) (*DiscoveryReport, error) {
	if r == nil {
		r = NewRunner(nil)
	}
	start := time.Now()
	days = r.Store.BestCacheDays(days)
	ds, err := r.LoadDataset(ctx, days, true)
	if err != nil {
		return nil, err
	}
	folds := BuildWFAFolds(ds.Timeline, proWFAFolds, 0.55, proHoldoutFrac, 0.02)
	holdStart, holdEnd := HoldoutWindow(ds.Timeline, proHoldoutFrac)

	grid := GenerateLeanGrid(days)
	label := "lean"
	switch mode {
	case "coarse":
		grid = GenerateCoarseGrid(days)
		label = "coarse"
	case "enriched":
		grid = GenerateEnrichedGrid(days)
		label = "enriched"
	}

	fmt.Printf("Discovery catalog: %d %s configs | %dd | %d symbols\n", len(grid), label, days, len(ds.All))

	var results []Result
	for i, cfg := range grid {
		res, err := r.RunDataset(ds, cfg)
		if err != nil {
			continue
		}
		results = append(results, *res)
		if (i+1)%12 == 0 || i+1 == len(grid) {
			fmt.Printf("  scanned %d/%d\n", i+1, len(grid))
		}
	}

	rep := &DiscoveryReport{
		Days: days, Symbols: len(ds.All), Bars: len(ds.Timeline),
		TotalRuns: len(results),
	}

	seen := map[string]bool{}
	for _, res := range results {
		if res.NetPnL <= 0 && res.OOSNetPnL <= 0 {
			continue
		}
		fp := TradeFingerprint(res)
		if fp != "" && seen[fp] {
			continue
		}
		if fp != "" {
			seen[fp] = true
		}

		oosPF := oosProfitFactor(res)
		passes, metrics := EvaluateWFA(&res, folds, 0)
		foldPnL := make([]float64, len(metrics))
		for i, m := range metrics {
			foldPnL[i] = m.OOSNetPnL
		}
		hold := res.OOSMetricsBetween(holdStart, holdEnd)
		sessPnL := res.SessionPnLBetween(holdStart, holdEnd)
		bestSess := bestSession(sessPnL)

		entry := CatalogEntry{
			Config: res.Config, NetPnL: res.NetPnL, OOSPnL: res.OOSNetPnL,
			OOSPF: oosPF, PF: res.ProfitFactor, MaxDD: res.MaxDrawdown,
			Trades: res.TradeCount, OOSTrades: res.OOSTrades, WinRate: res.WinRate,
			WFAFoldPnL: foldPnL, WFAPasses: passes,
			HoldoutPnL: hold.OOSNetPnL, HoldoutPF: hold.OOSPF,
			BestSession: bestSess,
		}
		entry.Tier, entry.ApplyWhen, entry.Caution = classifyCatalog(entry, passes, len(folds))

		if entry.Tier == TierD {
			continue
		}
		rep.Profitable++
		switch entry.Tier {
		case TierA:
			rep.TierA++
		case TierB:
			rep.TierB++
		case TierC:
			rep.TierC++
		}
		rep.Entries = append(rep.Entries, entry)
	}

	sort.Slice(rep.Entries, func(i, j int) bool {
		ti, tj := tierRank(rep.Entries[i].Tier), tierRank(rep.Entries[j].Tier)
		if ti != tj {
			return ti < tj
		}
		return rep.Entries[i].OOSPnL > rep.Entries[j].OOSPnL
	})
	rep.Elapsed = time.Since(start)
	return rep, nil
}

func tierRank(t CatalogTier) int {
	switch t {
	case TierA:
		return 0
	case TierB:
		return 1
	case TierC:
		return 2
	default:
		return 9
	}
}

func classifyCatalog(e CatalogEntry, wfaPasses, wfaTotal int) (CatalogTier, string, string) {
	cfg := e.Config
	pb := pbLabel(cfg.PlaybooksOnly)
	sess := sessLabel(cfg.SessionsOnly)

	if e.OOSPnL > 0 && e.OOSPF >= 1.05 && e.NetPnL > 0 && e.MaxDD < 100 {
		apply := fmt.Sprintf("%s playbooks | %s floor | u%d | t%d/day | %s sessions | default exit",
			pb, floorLabel(cfg.FloorOverride), cfg.UniverseTopN, cfg.maxTradesDay(), sess)
		caution := ""
		if e.HoldoutPnL < 10 {
			caution = fmt.Sprintf("holdout weak ($%.1f) — edge may be fading in recent weeks", e.HoldoutPnL)
		}
		if wfaPasses < 3 {
			caution += fmt.Sprintf("; WFA %d/%d — not stable every window", wfaPasses, wfaTotal)
		}
		return TierA, apply, strings.TrimPrefix(caution, "; ")
	}
	if e.OOSPnL > 0 && e.NetPnL > 0 {
		apply := fmt.Sprintf("Paper first: %s | %s | u%d | %s", pb, floorLabel(cfg.FloorOverride), cfg.UniverseTopN, sess)
		return TierB, apply, "moderate OOS — shadow before any capital"
	}
	if e.NetPnL > 0 && e.OOSPnL <= 0 {
		return TierC, "do not apply forward", "IS profit only — classic overfit signature"
	}
	return TierD, "", ""
}

func bestSession(m map[string]float64) string {
	best, val := "", 0.0
	for k, v := range m {
		if v > val {
			best, val = k, v
		}
	}
	if best == "" {
		return "n/a"
	}
	return fmt.Sprintf("%s ($%.1f)", best, val)
}

func FormatDiscoveryReport(rep *DiscoveryReport) string {
	if rep == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n=== STRATEGY CATALOG (historical research — not live approval) ===\n")
	b.WriteString(fmt.Sprintf("Data: %dd | %d symbols | %d bars | scanned %d configs in %s\n",
		rep.Days, rep.Symbols, rep.Bars, rep.TotalRuns, rep.Elapsed.Round(time.Second)))
	b.WriteString(fmt.Sprintf("Profitable: %d unique | Tier A: %d | Tier B: %d | Tier C (overfit): %d\n\n",
		rep.Profitable, rep.TierA, rep.TierB, rep.TierC))

	b.WriteString("Tiers:\n")
	b.WriteString("  A = OOS profitable + PF≥1.05 — candidate to apply when regime matches\n")
	b.WriteString("  B = OOS profitable — watchlist, paper first\n")
	b.WriteString("  C = IS only — archive, do not forward-apply\n\n")

	if len(rep.Entries) == 0 {
		b.WriteString("No profitable configs found in this grid.\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("%-3s %-40s %7s %7s %5s %5s %6s %s\n",
		"Tier", "Strategy", "Net", "OOS", "OOSPF", "WFA", "Hold", "Best session"))
	for _, e := range rep.Entries {
		wfa := fmt.Sprintf("%d/4", e.WFAPasses)
		b.WriteString(fmt.Sprintf("%-3s %-40s %7.1f %7.1f %5.2f %5s %6.1f %s\n",
			e.Tier, trunc(e.Config.Name, 40),
			e.NetPnL, e.OOSPnL, e.OOSPF, wfa, e.HoldoutPnL, e.BestSession))
	}

	b.WriteString("\n=== APPLY IN FUTURE (Tier A & B) ===\n")
	n := 0
	for _, e := range rep.Entries {
		if e.Tier != TierA && e.Tier != TierB {
			continue
		}
		n++
		b.WriteString(fmt.Sprintf("\n#%d [Tier %s] %s\n", n, e.Tier, e.Config.Name))
		b.WriteString(fmt.Sprintf("  PnL: net $%.1f | OOS $%.1f | PF %.2f | OOS PF %.2f | DD $%.1f | %d trades\n",
			e.NetPnL, e.OOSPnL, e.PF, e.OOSPF, e.MaxDD, e.Trades))
		b.WriteString(fmt.Sprintf("  WFA folds ($): %v\n", fmtFoldPnL(e.WFAFoldPnL)))
		b.WriteString(fmt.Sprintf("  Config: %s\n", e.ApplyWhen))
		if e.Caution != "" {
			b.WriteString(fmt.Sprintf("  Caution: %s\n", e.Caution))
		}
		b.WriteString("  → Load this config when: London/US active, u30 liquid alts, similar vol to mid-backtest window\n")
	}
	if n == 0 {
		b.WriteString("  No Tier A/B strategies. Expand grid with -discover=coarse or fix signal edge.\n")
	}

	b.WriteString("\n=== DO NOT APPLY (Tier C overfit) ===\n")
	for _, e := range rep.Entries {
		if e.Tier != TierC {
			continue
		}
		b.WriteString(fmt.Sprintf("  %s — net $%.1f but OOS $%.1f\n", e.Config.Name, e.NetPnL, e.OOSPnL))
	}
	return b.String()
}
