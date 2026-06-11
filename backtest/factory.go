package backtest

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type RankedStrategy struct {
	Rank       int
	Score      float64
	Config     RunConfig
	Result     Result
	Summary    string
	Promotable bool
}

func ExitPresets() map[string]ExitParams {
	return map[string]ExitParams{
		"default":      DefaultExitParams(),
		"conservative": {BEAtR: 0.75, PartialAtR: 1.5, PartialPct: 0.35, FullTPAtR: 3.0, StaleHours: 96, TrailATR: 2.0},
		"aggressive":   {BEAtR: 0.35, PartialAtR: 0.75, PartialPct: 0.5, FullTPAtR: 2.0, StaleHours: 48, TrailATR: 1.2},
		"tight":        {BEAtR: 0.4, PartialAtR: 0.8, PartialPct: 0.45, FullTPAtR: 1.8, StaleHours: 36, TrailATR: 1.0},
		"wide":         {BEAtR: 0.6, PartialAtR: 1.25, PartialPct: 0.3, FullTPAtR: 3.5, StaleHours: 120, TrailATR: 2.5},
		"runner":       {BEAtR: 0.5, PartialAtR: 1.0, PartialPct: 0.25, FullTPAtR: 4.0, StaleHours: 96, TrailATR: 2.0},
	}
}

func PlaybookSets() map[string][]string {
	return map[string][]string{
		"MOMENTUM_BURST":     {"MOMENTUM_BURST"},
		"SESSION_BREAKOUT":   {"SESSION_BREAKOUT"},
		"MEAN_REVERT_VWAP":   {"MEAN_REVERT_VWAP"},
		"MOMENTUM+SESSION":   {"MOMENTUM_BURST", "SESSION_BREAKOUT"},
		"MOMENTUM+REVERT":    {"MOMENTUM_BURST", "MEAN_REVERT_VWAP"},
		"SESSION+REVERT":     {"SESSION_BREAKOUT", "MEAN_REVERT_VWAP"},
		"FORCED_FLOW":        {"FORCED_FLOW_FADE"},
		"REVERT+FLOW":        {"MEAN_REVERT_VWAP", "FORCED_FLOW_FADE"},
		"SESSION+FLOW":       {"SESSION_BREAKOUT", "FORCED_FLOW_FADE"},
		"VOL_CLIMAX":         {"VOL_CLIMAX_FADE"},
		"REVERT+VOL":         {"MEAN_REVERT_VWAP", "VOL_CLIMAX_FADE"},
		"ALL":                {},
	}
}

// GenerateEnrichedGrid tests bar + Binance context playbooks (24 configs).
func GenerateEnrichedGrid(days int) []RunConfig {
	if days <= 0 {
		days = 60
	}
	return generateGrid(gridSpec{
		days: days,
		playbookNames: []string{
			"SESSION+REVERT", "FORCED_FLOW", "REVERT+FLOW", "SESSION+FLOW", "ALL",
		},
		floors:    []floorOpt{{"adapt", 0}, {"f55", 55}},
		universes: []int{30},
		maxTrades: []int{4},
		exitNames: []string{"default"},
		sessions: []sessOpt{
			{label: "all", val: nil},
			{label: "no_asia", val: []string{"london", "us", "late_us"}},
		},
	})
}

// GenerateLeanGrid is the pro thesis: 12 configs, one book family, default exit only.
func GenerateLeanGrid(days int) []RunConfig {
	if days <= 0 {
		days = 60
	}
	return generateGrid(gridSpec{
		days: days,
		playbookNames: []string{
			"SESSION+REVERT", "SESSION_BREAKOUT", "MEAN_REVERT_VWAP",
		},
		floors:    []floorOpt{{"adapt", 0}, {"f55", 55}},
		universes: []int{30},
		maxTrades: []int{4},
		exitNames: []string{"default"},
		sessions: []sessOpt{
			{label: "all", val: nil},
			{label: "no_asia", val: []string{"london", "us", "late_us"}},
		},
	})
}

// GenerateCoarseGrid is the wider search (56 configs) — use only after lean grid promotes.
func GenerateCoarseGrid(days int) []RunConfig {
	if days <= 0 {
		days = 60
	}
	return generateGrid(gridSpec{
		days: days,
		floors:    []floorOpt{{"adapt", 0}, {"f55", 55}},
		universes: []int{30},
		maxTrades: []int{4},
		exitNames: []string{"default", "aggressive"},
		sessions: []sessOpt{
			{label: "all", val: nil},
			{label: "no_asia", val: []string{"london", "us", "late_us"}},
		},
	})
}

func GenerateFactoryGrid(days int) []RunConfig {
	if days <= 0 {
		days = 60
	}
	return generateGrid(gridSpec{
		days: days,
		floors: []floorOpt{
			{"adapt", 0}, {"f58", 58}, {"f55", 55}, {"f52", 52},
		},
		universes: []int{30, 50},
		maxTrades: []int{2, 4, 6},
		exitNames: []string{"default", "conservative", "aggressive", "wide"},
		sessions: []sessOpt{
			{label: "all", val: nil},
			{label: "no_asia", val: []string{"london", "us", "late_us"}},
		},
	})
}

type floorOpt struct {
	label string
	val   int
}

type sessOpt struct {
	label string
	val   []string
}

type gridSpec struct {
	days          int
	playbookNames []string // empty = all playbooks
	floors        []floorOpt
	universes     []int
	maxTrades     []int
	exitNames     []string
	sessions      []sessOpt
}

func generateGrid(spec gridSpec) []RunConfig {
	pbSets := PlaybookSets()
	exits := ExitPresets()
	days := spec.days
	if days <= 0 {
		days = 60
	}

	var grid []RunConfig
	for pbName, pbs := range pbSets {
		if len(spec.playbookNames) > 0 && !containsStr(spec.playbookNames, pbName) {
			continue
		}
		for _, fl := range spec.floors {
			for _, uni := range spec.universes {
				for _, mt := range spec.maxTrades {
					for _, exitName := range spec.exitNames {
						exitP := exits[exitName]
						for _, sess := range spec.sessions {
							name := fmt.Sprintf("%s_%s_%s_u%d_t%d_%s",
								pbName, fl.label, exitName, uni, mt, sess.label)
							grid = append(grid, RunConfig{
								Name: name, Days: days, UniverseTopN: uni,
								PlaybooksOnly: append([]string(nil), pbs...),
								SessionsOnly: append([]string(nil), sess.val...),
								FloorOverride: fl.val, MaxTradesPerDay: mt,
								Exit: exitP, CachedOnly: true,
							})
						}
					}
				}
			}
		}
	}
	return grid
}

// RunFactory runs the full 1344-config grid (legacy brute force).
func RunFactory(ctx context.Context, r *Runner, days int, topN int) ([]RankedStrategy, error) {
	if r == nil {
		r = NewRunner(nil)
	}
	ds, err := r.LoadDataset(ctx, days, true)
	if err != nil {
		return nil, err
	}
	grid := GenerateFactoryGrid(days)
	fmt.Printf("Factory: %d strategies on %d symbols, %d bars\n",
		len(grid), len(ds.All), len(ds.Timeline))

	results := make([]Result, 0, len(grid))
	for i, cfg := range grid {
		res, err := r.RunDataset(ds, cfg)
		if err != nil {
			continue
		}
		results = append(results, *res)
		if (i+1)%50 == 0 {
			fmt.Printf("  ... %d/%d done\n", i+1, len(grid))
		}
	}
	allRanked := RankStrategies(results)
	for i := range allRanked {
		allRanked[i].Rank = i + 1
	}
	if topN > 0 && len(allRanked) > topN {
		return allRanked[:topN], nil
	}
	return allRanked, nil
}

func RankStrategies(results []Result) []RankedStrategy {
	out := make([]RankedStrategy, 0, len(results))
	for _, res := range results {
		oosPF := oosProfitFactor(res)
		score := strategyScore(res, oosPF)
		promo := res.OOSNetPnL > 0 && oosPF >= 1.05 && res.OOSTrades >= 8 && res.MaxDrawdown < 80
		summary := fmt.Sprintf("net=%.1f oos=%.1f pf=%.2f oosPF=%.2f dd=%.1f trades=%d oosT=%d wr=%.0f%%",
			res.NetPnL, res.OOSNetPnL, res.ProfitFactor, oosPF, res.MaxDrawdown,
			res.TradeCount, res.OOSTrades, res.WinRate)
		out = append(out, RankedStrategy{
			Score: score, Config: res.Config, Result: res,
			Summary: summary, Promotable: promo,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Result.OOSNetPnL > out[j].Result.OOSNetPnL
	})
	return out
}

func strategyScore(res Result, oosPF float64) float64 {
	// Pro desk: prioritize OOS edge, penalize drawdown and fee drag.
	score := res.OOSNetPnL*3.0 + res.NetPnL*0.5
	score += (oosPF - 1.0) * 25.0
	score += (res.ProfitFactor - 1.0) * 10.0
	score -= res.MaxDrawdown * 0.4
	score += res.ExpectancyR * 5.0
	if res.OOSTrades < 5 {
		score -= 50
	}
	return score
}

func oosProfitFactor(res Result) float64 {
	var wins, losses float64
	for _, t := range res.Trades {
		if !t.IsOOS {
			continue
		}
		if t.PnLUSD > 0 {
			wins += t.PnLUSD
		} else if t.PnLUSD < 0 {
			losses -= t.PnLUSD
		}
	}
	if losses <= 0 {
		if wins > 0 {
			return 99
		}
		return 0
	}
	return wins / losses
}

func FormatTopStrategies(ranked []RankedStrategy, promotableOnly bool) string {
	var b strings.Builder
	b.WriteString("\n=== TOP STRATEGIES (ranked by pro score: OOS PnL + PF − DD) ===\n")
	b.WriteString(fmt.Sprintf("%-4s %-6s %-48s %8s %8s %6s %6s %7s %s\n",
		"#", "Score", "Strategy", "NetPnL", "OOSPnL", "PF", "OOSPF", "MaxDD", "Promote"))
	n := 0
	for _, r := range ranked {
		if promotableOnly && !r.Promotable {
			continue
		}
		n++
		tag := ""
		if r.Promotable {
			tag = "YES"
		}
		b.WriteString(fmt.Sprintf("%-4d %-6.1f %-48s %8.2f %8.2f %6.2f %6.2f %7.1f %s\n",
			n, r.Score, trunc(r.Config.Name, 48), r.Result.NetPnL, r.Result.OOSNetPnL,
			r.Result.ProfitFactor, oosProfitFactor(r.Result), r.Result.MaxDrawdown, tag))
		if n >= 20 {
			break
		}
	}
	return b.String()
}

func FormatStrategyPlaybook(rows []RankedStrategy) string {
	var b strings.Builder
	b.WriteString("\n=== RECOMMENDED LIVE CONFIG (top promotable) ===\n")
	count := 0
	for _, r := range rows {
		if !r.Promotable {
			continue
		}
		count++
		c := r.Config
		b.WriteString(fmt.Sprintf("\n#%d score=%.1f %s\n", count, r.Score, c.Name))
		b.WriteString(fmt.Sprintf("  playbooks: %v\n", pbLabel(c.PlaybooksOnly)))
		b.WriteString(fmt.Sprintf("  floor: %s | universe: %d | max trades/day: %d\n",
			floorLabel(c.FloorOverride), c.UniverseTopN, c.maxTradesDay()))
		b.WriteString(fmt.Sprintf("  sessions: %s | exit: BE=%.2f partial=%.2fR tp=%.2fR\n",
			sessLabel(c.SessionsOnly), c.Exit.BEAtR, c.Exit.PartialAtR, c.Exit.FullTPAtR))
		b.WriteString(fmt.Sprintf("  backtest: %s\n", r.Summary))
		if count >= 5 {
			break
		}
	}
	if count == 0 {
		b.WriteString("  (none passed OOS profit + PF gates — see top 20 by score)\n")
	}
	return b.String()
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func pbLabel(pbs []string) string {
	if len(pbs) == 0 {
		return "ALL"
	}
	return strings.Join(pbs, "+")
}

func floorLabel(f int) string {
	if f == 0 {
		return "adaptive"
	}
	return fmt.Sprintf("%d", f)
}

func sessLabel(s []string) string {
	if len(s) == 0 {
		return "all"
	}
	return strings.Join(s, "+")
}

func containsStr(ss []string, x string) bool {
	for _, s := range ss {
		if s == x {
			return true
		}
	}
	return false
}
