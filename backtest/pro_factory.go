package backtest

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	proWFAFolds        = 4
	proWFAMinPasses    = 3
	proCoarseTopN      = 8
	proStressSlipMult  = 2.0
	proHoldoutFrac     = 0.85
	proHoldoutMinPnL   = 10.0
	proHoldoutMinPF    = 1.05
	proHoldoutMinTrades = 5
)

// ProRankedStrategy extends RankedStrategy with funnel-stage evidence.
type ProRankedStrategy struct {
	RankedStrategy
	WFAPasses         int
	WFATotal          int
	WFAFoldPnL        []float64
	StressNetPnL      float64
	HoldoutPnL        float64
	HoldoutPF         float64
	HoldoutTrades     int
	HoldoutBySession  map[string]float64
	DSR               float64
	Verdict           string
	DedupedFrom       int
}

// ProFactoryReport is the full funnel output.
type ProFactoryReport struct {
	DatasetDays    int
	DatasetSymbols int
	DatasetBars    int
	CoarseTotal    int
	SmokePassed    int
	DedupedCount   int
	WFASurvivors   int
	StressPassed   int
	HoldoutPassed  int
	Promotable     int
	Folds          []WFAFold
	HoldoutStart   time.Time
	Candidates     []ProRankedStrategy
	Elapsed        time.Duration
}

// RunProFactory executes lean grid → smoke → dedupe → WFA → stress → strict holdout.
func RunProFactory(ctx context.Context, r *Runner, days int, topN int) (*ProFactoryReport, error) {
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
	trialN := len(grid)
	fmt.Printf("Pro factory v2: %d lean configs on %dd / %d symbols / %d bars\n",
		trialN, days, len(ds.All), len(ds.Timeline))
	fmt.Printf("  deleted: aggressive exit, ALL/MOMENTUM combos, u50, extra floors\n")
	fmt.Printf("  WFA: %d folds need %d/4 (min $%.0f/fold) | holdout: tail %.0f%% need $%.0f PF>=%.2f\n",
		len(folds), proWFAMinPasses, proWFAMinFoldPnL, (1-proHoldoutFrac)*100, proHoldoutMinPnL, proHoldoutMinPF)

	report := &ProFactoryReport{
		DatasetDays:    days,
		DatasetSymbols: len(ds.All),
		DatasetBars:    len(ds.Timeline),
		CoarseTotal:    trialN,
		Folds:          folds,
		HoldoutStart:   holdStart,
	}

	var smokeResults []Result
	for i, cfg := range grid {
		res, err := r.RunDataset(ds, cfg)
		if err != nil {
			continue
		}
		if SmokeGate(*res) {
			smokeResults = append(smokeResults, *res)
		}
		if (i+1)%6 == 0 || i+1 == len(grid) {
			fmt.Printf("  lean %d/%d smoke passed: %d\n", i+1, len(grid), len(smokeResults))
		}
	}
	report.SmokePassed = len(smokeResults)
	if len(smokeResults) == 0 {
		report.Elapsed = time.Since(start)
		return report, nil
	}

	deduped := DedupeResults(smokeResults)
	report.DedupedCount = len(deduped)
	fmt.Printf("  deduped: %d → %d unique trade paths\n", len(smokeResults), len(deduped))

	ranked := RankStrategies(deduped)
	if len(ranked) > proCoarseTopN {
		ranked = ranked[:proCoarseTopN]
	}

	var candidates []ProRankedStrategy
	for _, rs := range ranked {
		passes, metrics := EvaluateWFA(&rs.Result, folds, proWFAMinPasses)
		if !wfaPassed(passes, len(folds), proWFAMinPasses) {
			continue
		}
		report.WFASurvivors++

		foldPnL := make([]float64, len(metrics))
		for i, m := range metrics {
			foldPnL[i] = m.OOSNetPnL
		}

		stressCfg := rs.Config
		stressCfg.SlippageMult = proStressSlipMult
		stressRes, err := r.RunDataset(ds, stressCfg)
		if err != nil || stressRes.NetPnL <= 0 {
			continue
		}
		report.StressPassed++

		hold := rs.Result.OOSMetricsBetween(holdStart, holdEnd)
		if !HoldoutGate(hold, proHoldoutMinPnL, proHoldoutMinPF, proHoldoutMinTrades) {
			continue
		}
		report.HoldoutPassed++

		sharpe := TradeSharpe(&rs.Result)
		dsr := DeflatedSharpeRatio(sharpe, rs.Result.TradeCount, trialN)

		pr := ProRankedStrategy{
			RankedStrategy:   rs,
			WFAPasses:        passes,
			WFATotal:         len(folds),
			WFAFoldPnL:       foldPnL,
			StressNetPnL:     stressRes.NetPnL,
			HoldoutPnL:       hold.OOSNetPnL,
			HoldoutPF:        hold.OOSPF,
			HoldoutTrades:    hold.OOSTrades,
			HoldoutBySession: rs.Result.SessionPnLBetween(holdStart, holdEnd),
			DSR:              dsr,
			DedupedFrom:      len(smokeResults) - len(deduped) + 1,
		}
		pr.Verdict = proVerdict(pr)
		if pr.Verdict == "PROMOTE" {
			report.Promotable++
		}
		candidates = append(candidates, pr)
	}

	sortProCandidates(candidates)
	if topN > 0 && len(candidates) > topN {
		candidates = candidates[:topN]
	}
	report.Candidates = candidates
	report.Elapsed = time.Since(start)
	return report, nil
}

func proVerdict(pr ProRankedStrategy) string {
	r := pr.Result
	oosPF := oosProfitFactor(r)
	holdOK := HoldoutGate(FoldMetrics{
		OOSNetPnL: pr.HoldoutPnL, OOSPF: pr.HoldoutPF, OOSTrades: pr.HoldoutTrades,
	}, proHoldoutMinPnL, proHoldoutMinPF, proHoldoutMinTrades)

	if holdOK && pr.WFAPasses >= proWFAMinPasses && pr.DSR > 0 &&
		r.OOSNetPnL > 0 && oosPF >= 1.05 && r.OOSTrades >= 8 &&
		r.MaxDrawdown < 80 && pr.StressNetPnL > 0 {
		return "PROMOTE"
	}
	if pr.WFAPasses >= proWFAMinPasses && pr.HoldoutPnL > 0 && pr.StressNetPnL > 0 && pr.DSR > -0.3 {
		return "SHADOW"
	}
	if pr.WFAPasses >= 2 {
		return "WATCH"
	}
	return "REJECT"
}

func sortProCandidates(c []ProRankedStrategy) {
	for i := 0; i < len(c); i++ {
		for j := i + 1; j < len(c); j++ {
			if proRankKey(c[j]) > proRankKey(c[i]) {
				c[i], c[j] = c[j], c[i]
			}
		}
	}
}

func proRankKey(p ProRankedStrategy) float64 {
	v := 0.0
	switch p.Verdict {
	case "PROMOTE":
		v += 1000
	case "SHADOW":
		v += 500
	case "WATCH":
		v += 100
	}
	v += p.DSR * 50
	v += p.HoldoutPnL * 2
	v += p.Score
	return v
}

func FormatProFactoryReport(rep *ProFactoryReport) string {
	if rep == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n=== PRO FACTORY v2 ===\n")
	b.WriteString(fmt.Sprintf("Dataset: %dd | %d symbols | %d bars | %s\n",
		rep.DatasetDays, rep.DatasetSymbols, rep.DatasetBars, rep.Elapsed.Round(time.Second)))
	b.WriteString(fmt.Sprintf("Funnel: %d lean → %d smoke → %d deduped → %d WFA → %d stress → %d holdout → %d PROMOTE\n",
		rep.CoarseTotal, rep.SmokePassed, rep.DedupedCount,
		rep.WFASurvivors, rep.StressPassed, rep.HoldoutPassed, rep.Promotable))

	if len(rep.Candidates) == 0 {
		b.WriteString("\nVERDICT: NO LIVE. Capital stays off.\n")
		b.WriteString("Holdout or DSR blocked everyone. Shadow nothing until holdout clears $10.\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("\n%-4s %-8s %-40s %7s %7s %6s %5s %7s\n",
		"#", "Verdict", "Strategy", "OOSPnL", "Hold", "DSR", "WFA", "Stress"))
	for i, c := range rep.Candidates {
		wfa := fmt.Sprintf("%d/%d", c.WFAPasses, c.WFATotal)
		b.WriteString(fmt.Sprintf("%-4d %-8s %-40s %7.1f %7.1f %6.2f %5s %7.1f\n",
			i+1, c.Verdict, trunc(c.Config.Name, 40),
			c.Result.OOSNetPnL, c.HoldoutPnL, c.DSR, wfa, c.StressNetPnL))
	}

	b.WriteString("\n=== HOLDOUT BY SESSION (top candidates) ===\n")
	for i, c := range rep.Candidates {
		if i >= 3 {
			break
		}
		b.WriteString(fmt.Sprintf("  %s: %s\n", trunc(c.Config.Name, 36), FormatSessionPnL(c.HoldoutBySession)))
	}

	b.WriteString("\n=== ACTION ===\n")
	shown := 0
	for _, c := range rep.Candidates {
		if c.Verdict != "PROMOTE" && c.Verdict != "SHADOW" {
			continue
		}
		shown++
		cfg := c.Config
		b.WriteString(fmt.Sprintf("\n#%d [%s] %s\n", shown, c.Verdict, cfg.Name))
		b.WriteString(fmt.Sprintf("  %s | floor %s | u%d | t%d | %s\n",
			pbLabel(cfg.PlaybooksOnly), floorLabel(cfg.FloorOverride),
			cfg.UniverseTopN, cfg.maxTradesDay(), sessLabel(cfg.SessionsOnly)))
		b.WriteString(fmt.Sprintf("  WFA: %v | holdout: $%.1f PF %.2f (%d tr) | stress: $%.1f | DSR: %.2f\n",
			fmtFoldPnL(c.WFAFoldPnL), c.HoldoutPnL, c.HoldoutPF, c.HoldoutTrades, c.StressNetPnL, c.DSR))
		if c.Verdict == "PROMOTE" {
			b.WriteString("  → Paper at 25% size. Funds SAFU until 2 weeks match.\n")
		} else {
			b.WriteString("  → Shadow only. No capital.\n")
		}
		if shown >= 2 {
			break
		}
	}
	if shown == 0 {
		b.WriteString("  All rejected for live. Wait.\n")
	}
	return b.String()
}

func fmtFoldPnL(v []float64) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = fmt.Sprintf("%.0f", x)
	}
	return strings.Join(parts, ", ")
}
