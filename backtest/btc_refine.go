package backtest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"encore.app/config"
	"encore.app/execution"
	"encore.app/market"
	"encore.app/model"
)

// BTCRefineRow is one strategy × timeframe × sizing scenario.
type BTCRefineRow struct {
	Strategy   string
	Timeframe  string
	Scenario   string
	RiskUSD    float64
	Leverage   int
	MaxPerDay  int
	RR         float64
	StopATR    float64
	NetPnL     float64
	OOSPnL     float64
	PF         float64
	MaxDD      float64
	Trades     int
	WinRate    float64
	ExpectR    float64
	Profitable bool
}

type btcSizing struct {
	riskUSD       float64
	activeCapital float64
	leverage      int
}

func provenBTCStrategies() []struct {
	name string
	fn   BTCStrategyFunc
} {
	return []struct {
		name string
		fn   BTCStrategyFunc
	}{
		{"S11_EMA_TREND", S11_H1_EMA_TREND},
		{"S4_SQUEEZE", S4_SQUEEZE_BREAK},
		{"S14_DONCHIAN", S14_DONCHIAN_VOL},
		{"S13_NY_CONT", S13_NY_CONTINUATION},
		{"S5_RSI2", S5_RSI2_SNAPBACK},
	}
}

// RunBTCRefine retests proven BTC lab winners on 15m/1h/4h with leverage sizing.
func RunBTCRefine(ctx context.Context, r *Runner, symbol string, days int) ([]BTCRefineRow, time.Duration, error) {
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
		return nil, 0, err
	}
	ds, err = FilterDataset(ds, []string{symbol})
	if err != nil {
		return nil, 0, err
	}
	bars5m := ds.All[symbol]
	if len(bars5m) < 500 {
		return nil, 0, fmt.Errorf("insufficient %s bars", symbol)
	}

	type tfSpec struct {
		label string
		n     int // 5m bars per candle
		hold  int
		cool  int
	}
	tfs := []tfSpec{
		{"15m", 3, 32, 4},
		{"1h", 12, 28, 3},
		{"4h", 48, 18, 2},
	}

	type scenario struct {
		label   string
		risk    float64
		lev     int
		maxDay  int
		rrSet   []float64
		stopSet []float64
	}
	scenarios := []scenario{
		{
			label: "baseline_1.25x1", risk: 1.25, lev: 1, maxDay: 4,
			rrSet: []float64{3.0, 3.5, 4.0}, stopSet: []float64{1.0, 1.4, 1.8, 2.2},
		},
		{
			label: "swing_5usd_5x_m2", risk: 5.0, lev: 5, maxDay: 2,
			rrSet: []float64{3.0, 3.5, 4.0, 5.0}, stopSet: []float64{1.0, 1.4, 1.8, 2.2, 2.8},
		},
		{
			label: "swing_8usd_5x_m1", risk: 8.0, lev: 5, maxDay: 1,
			rrSet: []float64{3.5, 4.0, 5.0}, stopSet: []float64{1.4, 1.8, 2.2, 2.8},
		},
	}

	var rows []BTCRefineRow
	proven := provenBTCStrategies()

	for _, tf := range tfs {
		bars := resampleBars(bars5m, tf.n)
		if len(bars) < 80 {
			continue
		}
		oosIdx := int(float64(len(bars)) * 0.7)
		fmt.Printf("  TF %s: %d bars\n", tf.label, len(bars))

		for _, sc := range scenarios {
			for _, p := range proven {
				var best BTCRefineRow
				for _, rr := range sc.rrSet {
					for _, stop := range sc.stopSet {
						par := btcLabParams{
							stopATR: stop, rr: rr, maxHold: tf.hold,
							cooldown: tf.cool, maxPerDay: sc.maxDay,
						}
						sz := btcSizing{
							riskUSD: sc.risk, activeCapital: config.ActiveCapitalUSD, leverage: sc.lev,
						}
						res := runOneBTCStrategySized(p.name, p.fn, bars, oosIdx, par, sz)
						if res.Trades < 3 {
							continue
						}
						row := BTCRefineRow{
							Strategy: p.name, Timeframe: tf.label, Scenario: sc.label,
							RiskUSD: sc.risk, Leverage: sc.lev, MaxPerDay: sc.maxDay,
							RR: rr, StopATR: stop,
							NetPnL: res.NetPnL, OOSPnL: res.OOSPnL, PF: res.PF,
							MaxDD: res.MaxDD, Trades: res.Trades, WinRate: res.WinRate,
							ExpectR: res.ExpectR, Profitable: res.Profitable,
						}
						if row.Profitable && (best.Trades == 0 || row.NetPnL > best.NetPnL) {
							best = row
						}
						if !row.Profitable && best.Trades == 0 && row.OOSPnL > best.OOSPnL {
							best = row
						}
					}
				}
				if best.Trades > 0 {
					rows = append(rows, best)
					st := "FAIL"
					if best.Profitable {
						st = "WIN"
					}
					fmt.Printf("    %s %s %s net=%.2f oos=%.2f tr=%d rr=%.1f stop=%.1f %s\n",
						p.name, tf.label, sc.label, best.NetPnL, best.OOSPnL, best.Trades,
						best.RR, best.StopATR, st)
				}
			}
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Scenario != rows[j].Scenario {
			return rows[i].Scenario < rows[j].Scenario
		}
		if rows[i].Timeframe != rows[j].Timeframe {
			return rows[i].Timeframe < rows[j].Timeframe
		}
		return rows[i].NetPnL > rows[j].NetPnL
	})
	return rows, time.Since(start), nil
}

func runOneBTCStrategySized(name string, fn BTCStrategyFunc, bars []Bar, oosIdx int, par btcLabParams, sz btcSizing) BTCLabResult {
	if sz.riskUSD <= 0 {
		sz.riskUSD = config.RiskPerTradeUSD
	}
	if sz.activeCapital <= 0 {
		sz.activeCapital = config.ActiveCapitalUSD
	}
	if sz.leverage <= 0 {
		sz.leverage = 1
	}

	var (
		pos       *btcPos
		trades    []Trade
		equity    = sz.activeCapital
		peak      = equity
		maxDD     float64
		coolLeft  int
		dayTrades = map[int]int{}
	)

	closeAt := func(px float64, reason string, at time.Time, isOOS bool) {
		if pos == nil {
			return
		}
		qty := pos.qty
		if qty <= 0 {
			qty = execution.RiskQuantity(sz.activeCapital, pos.riskUSD, pos.entry, pos.stop, sz.leverage)
		}
		if qty <= 0 {
			pos = nil
			return
		}
		var pnl float64
		if pos.side == model.SideLong {
			pnl = (px - pos.entry) * qty
		} else {
			pnl = (pos.entry - px) * qty
		}
		fee := (pos.entry*qty + px*qty) * takerFeeBps / 10000
		slip := px * qty * slippageBps / 10000
		net := pnl - fee - slip
		rMult := 0.0
		if pos.riskUSD > 0 {
			rMult = pnl / pos.riskUSD
		}
		trades = append(trades, Trade{
			Symbol: "BTCUSDT", Side: pos.side, Playbook: name,
			EntryTime: pos.entryTime, ExitTime: at,
			EntryPx: pos.entry, ExitPx: px, RMultiple: rMult,
			PnLUSD: net, FeesUSD: fee + slip, Reason: reason, IsOOS: isOOS,
		})
		equity += net
		if equity > peak {
			peak = equity
		}
		if dd := peak - equity; dd > maxDD {
			maxDD = dd
		}
		pos = nil
		coolLeft = par.cooldown
	}

	warmup := 50
	if len(bars) < 200 {
		warmup = 30
	}

	for i := warmup; i < len(bars); i++ {
		b := bars[i]
		isOOS := i >= oosIdx
		yd := b.CloseTime.UTC().YearDay() + b.CloseTime.UTC().Year()*400
		if coolLeft > 0 {
			coolLeft--
		}

		if pos != nil {
			pos.barsHeld++
			hitStop := (pos.side == model.SideLong && b.Low <= pos.stop) ||
				(pos.side == model.SideShort && b.High >= pos.stop)
			hitTP := (pos.side == model.SideLong && b.High >= pos.target) ||
				(pos.side == model.SideShort && b.Low <= pos.target)
			if hitStop {
				px := pos.stop
				if pos.side == model.SideLong {
					px *= 1 - slippageBps/10000
				} else {
					px *= 1 + slippageBps/10000
				}
				closeAt(px, "stop", b.CloseTime, isOOS)
				continue
			}
			if hitTP {
				closeAt(pos.target, "target", b.CloseTime, isOOS)
				continue
			}
			if pos.barsHeld >= par.maxHold {
				closeAt(b.Close, "time", b.CloseTime, isOOS)
				continue
			}
			continue
		}

		if coolLeft > 0 {
			continue
		}
		if par.maxPerDay > 0 && dayTrades[yd] >= par.maxPerDay {
			continue
		}
		side, ok := fn(bars, i)
		if !ok {
			continue
		}
		c := barsToCandles(bars, i)
		atr := market.ATR(c, 14)
		if atr <= 0 {
			continue
		}
		entry := b.Close
		stopMult := par.stopATR
		if stopMult <= 0 {
			stopMult = 1.2
		}
		rr := par.rr
		if rr <= 0 {
			rr = 2.0
		}
		stopDist := clampF(atr*stopMult, entry*0.0015, entry*0.025)
		var stop, target float64
		if side == model.SideLong {
			stop = entry - stopDist
			target = entry + stopDist*rr
		} else {
			stop = entry + stopDist
			target = entry - stopDist*rr
		}
		qty := execution.RiskQuantity(sz.activeCapital, sz.riskUSD, entry, stop, sz.leverage)
		pos = &btcPos{
			side: side, entry: entry, stop: stop, target: target,
			riskUSD: sz.riskUSD, entryTime: b.CloseTime, qty: qty,
		}
		dayTrades[yd]++
	}

	var net, oosNet, wins, losses, oosWins, oosLosses, sumR float64
	for _, t := range trades {
		net += t.PnLUSD
		sumR += t.RMultiple
		if t.PnLUSD > 0 {
			wins += t.PnLUSD
		} else if t.PnLUSD < 0 {
			losses -= t.PnLUSD
		}
		if t.IsOOS {
			oosNet += t.PnLUSD
			if t.PnLUSD > 0 {
				oosWins += t.PnLUSD
			} else if t.PnLUSD < 0 {
				oosLosses -= t.PnLUSD
			}
		}
	}
	pf := 0.0
	if losses > 0 {
		pf = wins / losses
	}
	wr := 0.0
	if len(trades) > 0 {
		w := 0
		for _, t := range trades {
			if t.PnLUSD > 0 {
				w++
			}
		}
		wr = float64(w) / float64(len(trades)) * 100
	}
	expR := 0.0
	if len(trades) > 0 {
		expR = sumR / float64(len(trades))
	}
	return BTCLabResult{
		Name: name, NetPnL: net, OOSPnL: oosNet, PF: pf,
		MaxDD: maxDD, Trades: len(trades), WinRate: wr, ExpectR: expR,
		Profitable: net > 0 && oosNet > 0,
	}
}

func FormatBTCRefine(symbol string, rows []BTCRefineRow, days int, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n=== BTC REFINE — proven strategies × TF × sizing on %s ===\n", symbol))
	b.WriteString(fmt.Sprintf("%dd | %s | 5 proven strategies | 15m / 1h / 4h\n\n", days, elapsed.Round(time.Second)))

	// Best per scenario × TF
	type key struct{ sc, tf string }
	best := map[key]BTCRefineRow{}
	for _, r := range rows {
		k := key{r.Scenario, r.Timeframe}
		cur, ok := best[k]
		if !ok || (r.Profitable && (!cur.Profitable || r.NetPnL > cur.NetPnL)) ||
			(!cur.Profitable && r.NetPnL > cur.NetPnL) {
			best[k] = r
		}
	}

	b.WriteString("--- Best strategy per timeframe × scenario ---\n")
	b.WriteString(fmt.Sprintf("%-10s %-22s %-6s %-8s %6s %6s %4s %5s %5s %s\n",
		"TF", "Scenario", "Risk", "Lev", "Net", "OOS", "Tr", "RR", "Stop", "Strategy"))
	for _, tf := range []string{"15m", "1h", "4h"} {
		for _, sc := range []string{"baseline_1.25x1", "swing_5usd_5x_m2", "swing_8usd_5x_m1"} {
			r, ok := best[key{sc, tf}]
			if !ok {
				continue
			}
			st := "FAIL"
			if r.Profitable {
				st = "WIN"
			}
			b.WriteString(fmt.Sprintf("%-10s %-22s $%-5.0f %dx      %6.2f %6.2f %4d %5.1f %5.1f %s [%s]\n",
				r.Timeframe, r.Scenario, r.RiskUSD, r.Leverage,
				r.NetPnL, r.OOSPnL, r.Trades, r.RR, r.StopATR, r.Strategy, st))
		}
	}

	b.WriteString("\n--- All profitable configs (net + OOS green) ---\n")
	winN := 0
	sorted := append([]BTCRefineRow(nil), rows...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].NetPnL > sorted[j].NetPnL })
	for _, r := range sorted {
		if !r.Profitable {
			continue
		}
		winN++
		b.WriteString(fmt.Sprintf("#%d %s | %s | %s | $%.0f risk %dx lev max%d/day | rr%.1f stop%.1f\n",
			winN, r.Strategy, r.Timeframe, r.Scenario, r.RiskUSD, r.Leverage, r.MaxPerDay, r.RR, r.StopATR))
		b.WriteString(fmt.Sprintf("   Net $%.2f | OOS $%.2f | PF %.2f | DD $%.0f | %d tr WR %.0f%%\n",
			r.NetPnL, r.OOSPnL, r.PF, r.MaxDD, r.Trades, r.WinRate))
	}
	if winN == 0 {
		b.WriteString("  None — see closest by net in full table below.\n")
	}

	b.WriteString("\n--- Full grid (best param per strategy×TF×scenario) ---\n")
	b.WriteString(fmt.Sprintf("%-14s %-5s %-20s %6s %6s %4s %5s %5s %4s\n",
		"Strategy", "TF", "Scenario", "Net", "OOS", "Tr", "RR", "Stop", "OK"))
	for _, r := range rows {
		ok := "no"
		if r.Profitable {
			ok = "YES"
		}
		b.WriteString(fmt.Sprintf("%-14s %-5s %-20s %6.2f %6.2f %4d %5.1f %5.1f %4s\n",
			r.Strategy, r.Timeframe, r.Scenario, r.NetPnL, r.OOSPnL, r.Trades, r.RR, r.StopATR, ok))
	}

	b.WriteString("\n--- Takeaways ---\n")
	b.WriteString("• Leverage scales notional (fees too) — capped at active_capital × leverage.\n")
	b.WriteString("• Fewer trades/day (1-2) + wider stops on 4h targets quality over quantity.\n")
	b.WriteString("• Compare baseline_1.25x1 (prior lab) vs swing_5usd_5x_m2 vs swing_8usd_5x_m1.\n")
	return b.String()
}
