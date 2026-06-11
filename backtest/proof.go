package backtest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ProofRow is one backtested config with full evidence.
type ProofRow struct {
	Name           string
	Playbooks      string
	Sessions       string
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
	HoldoutTrades  int
	TopPlaybook    string
	TopPlaybookPnL float64
	TopSession     string
	TopSessionPnL  float64
	StressNetPnL   float64
	Verdict        string
	Deduped        bool
}

// ProofReport is the profit proof run output.
type ProofReport struct {
	Days       int
	Symbols    int
	Bars       int
	ContextSyms int
	Rows       []ProofRow
	Elapsed    time.Duration
}

// proofConfigs are the only configs worth testing — no legacy factorial.
func proofConfigs(days int) []RunConfig {
	exits := ExitPresets()
	def := exits["default"]
	type pb struct{ name string; pbs []string }
	books := []pb{
		{"SESSION+REVERT", []string{"SESSION_BREAKOUT", "MEAN_REVERT_VWAP"}},
		{"SESSION_SOLO", []string{"SESSION_BREAKOUT"}},
		{"REVERT_SOLO", []string{"MEAN_REVERT_VWAP"}},
		{"MOMENTUM_SOLO", []string{"MOMENTUM_BURST"}},
		{"FORCED_FLOW", []string{"FORCED_FLOW_FADE"}},
		{"REVERT+FLOW", []string{"MEAN_REVERT_VWAP", "FORCED_FLOW_FADE"}},
		{"SESSION+FLOW", []string{"SESSION_BREAKOUT", "FORCED_FLOW_FADE"}},
		{"ALL", nil},
	}
	floors := []struct {
		label string
		val   int
	}{{"adapt", 0}, {"f55", 55}}
	sessions := []sessOpt{
		{"all", nil},
		{"no_asia", []string{"london", "us", "late_us"}},
	}
	var out []RunConfig
	for _, b := range books {
		for _, fl := range floors {
			for _, sess := range sessions {
				name := fmt.Sprintf("%s_%s_u30_t4_%s", b.name, fl.label, sess.label)
				out = append(out, RunConfig{
					Name: name, Days: days, UniverseTopN: 30,
					PlaybooksOnly: append([]string(nil), b.pbs...),
					SessionsOnly:  append([]string(nil), sess.val...),
					FloorOverride: fl.val, MaxTradesPerDay: 4,
					Exit: def, CachedOnly: true,
				})
			}
		}
	}
	return out
}

// RunProfitProof backtests the focused proof grid and ranks by evidence.
func RunProfitProof(ctx context.Context, r *Runner, days int) (*ProofReport, error) {
	if r == nil {
		r = NewRunner(nil)
	}
	start := time.Now()
	days = r.Store.BestCacheDays(days)
	ds, err := r.LoadDataset(ctx, days, true)
	if err != nil {
		return nil, err
	}
	_, ctxN := r.Store.HasContextBatch(days, 1)
	folds := BuildWFAFolds(ds.Timeline, 4, 0.55, 0.85, 0.02)
	holdStart, holdEnd := HoldoutWindow(ds.Timeline, 0.85)

	cfgs := proofConfigs(days)
	fmt.Printf("Profit proof: %d focused configs | %dd | %d symbols | context on %d symbols\n",
		len(cfgs), days, len(ds.All), ctxN)

	var results []Result
	for i, cfg := range cfgs {
		res, err := r.RunDataset(ds, cfg)
		if err != nil {
			continue
		}
		results = append(results, *res)
		fmt.Printf("  [%d/%d] %s net=%.1f oos=%.1f trades=%d\n",
			i+1, len(cfgs), cfg.Name, res.NetPnL, res.OOSNetPnL, res.TradeCount)
	}

	seen := map[string]string{}
	var rows []ProofRow
	for _, res := range results {
		fp := TradeFingerprint(res)
		deduped := false
		if fp != "" {
			if prev, ok := seen[fp]; ok {
				deduped = true
				if res.OOSNetPnL <= 0 {
					continue
				}
				_ = prev
			} else {
				seen[fp] = res.Config.Name
			}
		}
		if res.NetPnL <= 0 && res.OOSNetPnL <= 0 {
			continue
		}

		passes, metrics := EvaluateWFA(&res, folds, 3)
		foldPnL := make([]float64, len(metrics))
		for i, m := range metrics {
			foldPnL[i] = m.OOSNetPnL
		}
		hold := res.OOSMetricsBetween(holdStart, holdEnd)
		stressCfg := res.Config
		stressCfg.SlippageMult = 2.0
		stressRes, _ := r.RunDataset(ds, stressCfg)
		stressNet := 0.0
		if stressRes != nil {
			stressNet = stressRes.NetPnL
		}

		topPB, topPBPnL := topKeyPnL(res.Trades, true)
		topSess, topSessPnL := topKeyPnL(res.Trades, false)

		row := ProofRow{
			Name: res.Config.Name, Playbooks: pbLabel(res.Config.PlaybooksOnly),
			Sessions: sessLabel(res.Config.SessionsOnly),
			NetPnL: res.NetPnL, OOSPnL: res.OOSNetPnL, OOSPF: oosProfitFactor(res),
			PF: res.ProfitFactor, MaxDD: res.MaxDrawdown, Trades: res.TradeCount,
			OOSTrades: res.OOSTrades, WinRate: res.WinRate, WFAFoldPnL: foldPnL,
			WFAPasses: passes, HoldoutPnL: hold.OOSNetPnL, HoldoutPF: hold.OOSPF,
			HoldoutTrades: hold.OOSTrades, TopPlaybook: topPB, TopPlaybookPnL: topPBPnL,
			TopSession: topSess, TopSessionPnL: topSessPnL, StressNetPnL: stressNet,
			Deduped: deduped,
		}
		row.Verdict = proofVerdict(row)
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		vi, vj := verdictRank(rows[i].Verdict), verdictRank(rows[j].Verdict)
		if vi != vj {
			return vi < vj
		}
		return rows[i].OOSPnL > rows[j].OOSPnL
	})

	return &ProofReport{
		Days: days, Symbols: len(ds.All), Bars: len(ds.Timeline),
		ContextSyms: ctxN, Rows: rows, Elapsed: time.Since(start),
	}, nil
}

func proofVerdict(r ProofRow) string {
	if r.OOSPnL > 0 && r.OOSPF >= 1.05 && r.NetPnL > 0 && r.MaxDD < 100 &&
		r.WFAPasses >= 3 && r.HoldoutPnL >= 10 && r.HoldoutPF >= 1.05 && r.StressNetPnL > 0 {
		return "PROVEN_LIVE"
	}
	if r.OOSPnL > 0 && r.OOSPF >= 1.05 && r.NetPnL > 0 && r.MaxDD < 100 && r.StressNetPnL > 0 {
		return "PROVEN_HISTORICAL"
	}
	if r.OOSPnL > 0 && r.NetPnL > 0 {
		return "MARGINAL"
	}
	return "FAIL"
}

func verdictRank(v string) int {
	switch v {
	case "PROVEN_LIVE":
		return 0
	case "PROVEN_HISTORICAL":
		return 1
	case "MARGINAL":
		return 2
	default:
		return 9
	}
}

func topKeyPnL(trades []Trade, byPlaybook bool) (string, float64) {
	m := map[string]float64{}
	for _, t := range trades {
		k := t.Session
		if byPlaybook {
			k = t.Playbook
		}
		m[k] += t.PnLUSD
	}
	bestK, bestV := "", 0.0
	for k, v := range m {
		if v > bestV {
			bestK, bestV = k, v
		}
	}
	return bestK, bestV
}

func FormatProofReport(rep *ProofReport) string {
	if rep == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n=== PROFIT PROOF REPORT ===\n")
	b.WriteString(fmt.Sprintf("Data: %dd | %d symbols | %d bars | Binance context: %d symbols | %s\n\n",
		rep.Days, rep.Symbols, rep.Bars, rep.ContextSyms, rep.Elapsed.Round(time.Second)))

	b.WriteString("Verdicts:\n")
	b.WriteString("  PROVEN_LIVE        = OOS+PF + WFA 3/4 + holdout $10+ + stress positive\n")
	b.WriteString("  PROVEN_HISTORICAL  = OOS+PF + stress positive (paper when regime matches)\n")
	b.WriteString("  MARGINAL           = profitable but weak holdout/WFA/stress\n\n")

	if len(rep.Rows) == 0 {
		b.WriteString("No profitable configs in proof grid.\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("%-6s %-32s %7s %7s %5s %5s %6s %5s %7s %s\n",
		"Grade", "Config", "Net", "OOS", "OOSPF", "WFA", "Hold", "Trades", "Stress", "Top edge"))
	for _, r := range rep.Rows {
		wfa := fmt.Sprintf("%d/4", r.WFAPasses)
		tag := ""
		if r.Deduped {
			tag = " [dup]"
		}
		b.WriteString(fmt.Sprintf("%-6s %-32s %7.1f %7.1f %5.2f %5s %6.1f %5d %7.1f %s%s\n",
			r.Verdict, trunc(r.Name, 32), r.NetPnL, r.OOSPnL, r.OOSPF, wfa,
			r.HoldoutPnL, r.Trades, r.StressNetPnL, r.TopPlaybook, tag))
	}

	b.WriteString("\n=== DETAIL: PROVEN & MARGINAL ===\n")
	for i, r := range rep.Rows {
		if r.Verdict == "FAIL" {
			continue
		}
		b.WriteString(fmt.Sprintf("\n#%d [%s] %s\n", i+1, r.Verdict, r.Name))
		b.WriteString(fmt.Sprintf("  Playbooks: %s | Sessions: %s | floor in name\n", r.Playbooks, r.Sessions))
		b.WriteString(fmt.Sprintf("  Net $%.1f | OOS $%.1f | PF %.2f | OOS PF %.2f | DD $%.1f | WR %.0f%%\n",
			r.NetPnL, r.OOSPnL, r.PF, r.OOSPF, r.MaxDD, r.WinRate))
		b.WriteString(fmt.Sprintf("  WFA folds ($): %v | Holdout: $%.1f PF %.2f (%d tr)\n",
			fmtFoldPnL(r.WFAFoldPnL), r.HoldoutPnL, r.HoldoutPF, r.HoldoutTrades))
		b.WriteString(fmt.Sprintf("  Stress (2x slip): $%.1f | Top playbook: %s ($%.1f) | Top session: %s ($%.1f)\n",
			r.StressNetPnL, r.TopPlaybook, r.TopPlaybookPnL, r.TopSession, r.TopSessionPnL))
	}

	b.WriteString("\n=== FAILED (no OOS edge) ===\n")
	for _, r := range rep.Rows {
		if r.Verdict != "FAIL" {
			continue
		}
		b.WriteString(fmt.Sprintf("  %s — net $%.1f oos $%.1f\n", r.Name, r.NetPnL, r.OOSPnL))
	}
	return b.String()
}
