package backtest

import (
	"testing"
	"time"
)

func TestGenerateLeanGridCount(t *testing.T) {
	grid := GenerateLeanGrid(60)
	if len(grid) != 12 {
		t.Fatalf("lean grid: got %d want 12", len(grid))
	}
	for _, c := range grid {
		if c.Exit.BEAtR == 0.35 {
			t.Fatalf("aggressive exit should be deleted: %s", c.Name)
		}
	}
	full := GenerateFactoryGrid(60)
	if len(full) != 1344 {
		t.Fatalf("full grid: got %d want 1344", len(full))
	}
}

func TestBuildWFAFolds(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var timeline []time.Time
	for i := 0; i < 1000; i++ {
		timeline = append(timeline, start.Add(time.Duration(i*5)*time.Minute))
	}
	folds := BuildWFAFolds(timeline, 4, 0.55, 0.85, 0.02)
	if len(folds) < 3 {
		t.Fatalf("expected >=3 folds, got %d", len(folds))
	}
}

func TestSmokeGateStrict(t *testing.T) {
	if SmokeGate(Result{TradeCount: 10, ProfitFactor: 1.1, MaxDrawdown: 50, OOSNetPnL: -1}) {
		t.Fatal("negative OOS should fail")
	}
	if !SmokeGate(Result{TradeCount: 10, ProfitFactor: 1.1, MaxDrawdown: 50, OOSTrades: 5, OOSNetPnL: 10}) {
		t.Fatal("healthy OOS should pass")
	}
}

func TestHoldoutGate(t *testing.T) {
	if HoldoutGate(FoldMetrics{OOSNetPnL: 2, OOSPF: 2, OOSTrades: 36}, 10, 1.05, 5) {
		t.Fatal("$2 holdout should fail $10 gate")
	}
	if !HoldoutGate(FoldMetrics{OOSNetPnL: 15, OOSPF: 1.1, OOSTrades: 8}, 10, 1.05, 5) {
		t.Fatal("strong holdout should pass")
	}
}

func TestDedupeResults(t *testing.T) {
	mk := func(sym string, pnl float64) Result {
		return Result{
			NetPnL: pnl, TradeCount: 1,
			Trades: []Trade{{Symbol: sym, Playbook: "X", EntryTime: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), PnLUSD: pnl}},
		}
	}
	d := DedupeResults([]Result{mk("BTCUSDT", 10), mk("BTCUSDT", 5)})
	if len(d) != 1 {
		t.Fatalf("dedupe want 1 got %d", len(d))
	}
}

func TestDeflatedSharpePenalty(t *testing.T) {
	base := DeflatedSharpeRatio(1.5, 50, 1)
	penalized := DeflatedSharpeRatio(1.5, 50, 12)
	if penalized >= base {
		t.Fatalf("DSR should penalize trials: base=%.3f penalized=%.3f", base, penalized)
	}
}
