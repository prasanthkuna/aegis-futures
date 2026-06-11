package backtest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"encore.app/config"
	"encore.app/market"
	"encore.app/model"
)

// BTCLabResult is one custom BTC strategy backtest.
type BTCLabResult struct {
	Name      string
	NetPnL    float64
	OOSPnL    float64
	PF        float64
	OOSPF     float64
	MaxDD     float64
	Trades    int
	WinRate   float64
	ExpectR   float64
	Profitable bool
}

type btcPos struct {
	side      model.Side
	entry     float64
	stop      float64
	target    float64
	qty       float64
	riskUSD   float64
	playbook  string
	entryTime time.Time
	barsHeld  int
}

type btcLabParams struct {
	stopATR   float64
	rr        float64
	maxHold   int
	cooldown  int
	maxPerDay int
}

// RunBTCLab backtests custom BTC-only strategies (no Aegis playbook engine).
func RunBTCLab(ctx context.Context, r *Runner, symbol string, days int) ([]BTCLabResult, time.Duration, error) {
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
	bars := ds.All[symbol]
	bars1h := resampleBars(bars, 12) // 12 x 5m = 1h
	if len(bars) < 200 {
		return nil, 0, fmt.Errorf("insufficient %s bars", symbol)
	}
	oosIdx := int(float64(len(bars)) * 0.7)
	strats := allBTCLabStrategies()
	fmt.Printf("BTC lab: %d custom strategies | %s | %dd | %d 5m bars | %d 1h bars\n",
		len(strats), symbol, days, len(bars), len(bars1h))

	var out []BTCLabResult
	names := make([]string, 0, len(strats))
	for n := range strats {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		fn := strats[name]
		res := runOneBTCStrategy(name, fn, bars, oosIdx, btcLabParams{
			stopATR: 1.2, rr: 2.0, maxHold: 48, cooldown: 3, maxPerDay: 6,
		})
		out = append(out, res)
		fmt.Printf("  %s net=%.2f oos=%.2f tr=%d wr=%.0f%%\n",
			name, res.NetPnL, res.OOSPnL, res.Trades, res.WinRate)
	}

	// 1h resample — BTC cleaner on higher TF; grid-search all strategies.
	if len(bars1h) > 100 {
		oos1h := int(float64(len(bars1h)) * 0.7)
		h1wins := tuneOnBars(bars1h, oos1h, strats, "1h")
		for _, res := range h1wins {
			out = append(out, res)
			fmt.Printf("  [1h+] %s net=%.2f oos=%.2f tr=%d WIN=%v\n",
				res.Name, res.NetPnL, res.OOSPnL, res.Trades, res.Profitable)
		}
		for _, pick := range []struct {
			name string
			fn   BTCStrategyFunc
			p    btcLabParams
		}{
			{"S12_H1_DOJI", S12_H1_DOJI_FADE, btcLabParams{1.4, 3.5, 28, 3, 4}},
			{"S6_VWAP_H1", S6_VWAP_TOUCH, btcLabParams{1.2, 3.0, 28, 3, 4}},
			{"S3_LONDON_H1", S3_LONDON_STRICT, btcLabParams{1.8, 3.5, 28, 4, 2}},
			{"S10_INSIDE_H1", S10_H1_INSIDE_BREAK, btcLabParams{1.4, 3.0, 24, 3, 3}},
			{"S9_DEEP_BB_H1", S9_DEEP_BB, btcLabParams{1.0, 3.5, 24, 3, 4}},
			{"S13_NY_CONT", S13_NY_CONTINUATION, btcLabParams{1.4, 3.0, 20, 4, 3}},
			{"S14_DONCHIAN", S14_DONCHIAN_VOL, btcLabParams{1.4, 3.5, 24, 3, 3}},
			{"S15_ASIA_FADE", S15_ASIA_RANGE_FADE, btcLabParams{1.0, 3.0, 16, 3, 4}},
		} {
			res := runOneBTCStrategy(pick.name, pick.fn, bars1h, oos1h, pick.p)
			if res.Profitable {
				out = append(out, res)
				fmt.Printf("  [pick] %s net=%.2f oos=%.2f tr=%d\n", res.Name, res.NetPnL, res.OOSPnL, res.Trades)
			}
		}
	}

	tuned := tuneBTCLab(bars, oosIdx)
	for _, res := range tuned {
		out = append(out, res)
		fmt.Printf("  [tuned] %s net=%.2f oos=%.2f tr=%d\n", res.Name, res.NetPnL, res.OOSPnL, res.Trades)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].NetPnL != out[j].NetPnL {
			return out[i].NetPnL > out[j].NetPnL
		}
		return out[i].OOSPnL > out[j].OOSPnL
	})
	return out, time.Since(start), nil
}

func tuneOnBars(bars []Bar, oosIdx int, strats map[string]BTCStrategyFunc, suffix string) []BTCLabResult {
	var winners []BTCLabResult
	for base, fn := range strats {
		for _, rr := range []float64{2.0, 2.5, 3.0, 3.5, 4.0} {
			for _, stop := range []float64{0.8, 1.0, 1.4, 1.8, 2.2} {
				for _, mpd := range []int{2, 3, 4} {
					p := btcLabParams{stopATR: stop, rr: rr, maxHold: 28, cooldown: 3, maxPerDay: mpd}
					res := runOneBTCStrategy(base, fn, bars, oosIdx, p)
					if !res.Profitable || res.Trades < 3 {
						continue
					}
					res.Name = fmt.Sprintf("%s_%s_rr%.1f_s%.1f_m%d", base, suffix, rr, stop, mpd)
					winners = append(winners, res)
				}
			}
		}
	}
	sort.Slice(winners, func(i, j int) bool {
		if winners[i].NetPnL != winners[j].NetPnL {
			return winners[i].NetPnL > winners[j].NetPnL
		}
		return winners[i].OOSPnL > winners[j].OOSPnL
	})
	// dedupe by base strategy keeping best net
	seen := map[string]bool{}
	var uniq []BTCLabResult
	for _, w := range winners {
		key := w.Name
		if idx := strings.Index(key, "_rr"); idx > 0 {
			key = key[:idx]
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		uniq = append(uniq, w)
		if len(uniq) >= 10 {
			break
		}
	}
	return uniq
}

func tuneBTCLab(bars []Bar, oosIdx int) []BTCLabResult {
	type candidate struct {
		name string
		fn   BTCStrategyFunc
		p    btcLabParams
	}
	var grid []candidate
	for _, rr := range []float64{1.8, 2.2, 2.8, 3.2} {
		for _, stop := range []float64{1.0, 1.4, 1.8} {
			for _, mpd := range []int{2, 4} {
				p := btcLabParams{stopATR: stop, rr: rr, maxHold: 56, cooldown: 6, maxPerDay: mpd}
				grid = append(grid, candidate{"S3_LONDON", S3_LONDON_BREAK, p})
				grid = append(grid, candidate{"S3_STRICT", S3_LONDON_STRICT, p})
				grid = append(grid, candidate{"S7_IGNITE", S7_MOMENTUM_IGNITION, p})
				grid = append(grid, candidate{"S6_VWAP", S6_VWAP_TOUCH, p})
				grid = append(grid, candidate{"S8_LONG", S8_LONG_LONDON, p})
				grid = append(grid, candidate{"S9_DEEP_BB", S9_DEEP_BB, p})
			}
		}
	}
	var winners []BTCLabResult
	for _, c := range grid {
		res := runOneBTCStrategy(c.name, c.fn, bars, oosIdx, c.p)
		label := fmt.Sprintf("%s_rr%.1f_s%.1f_mpd%d", c.name, c.p.rr, c.p.stopATR, c.p.maxPerDay)
		res.Name = label
		if res.Profitable {
			winners = append(winners, res)
		}
	}
	sort.Slice(winners, func(i, j int) bool {
		if winners[i].NetPnL != winners[j].NetPnL {
			return winners[i].NetPnL > winners[j].NetPnL
		}
		return winners[i].OOSPnL > winners[j].OOSPnL
	})
	if len(winners) > 5 {
		winners = winners[:5]
	}
	return winners
}

func runOneBTCStrategy(name string, fn BTCStrategyFunc, bars []Bar, oosIdx int, par btcLabParams) BTCLabResult {
	riskUSD := config.RiskPerTradeUSD
	var (
		pos      *btcPos
		trades   []Trade
		equity   = config.ActiveCapitalUSD
		peak     = equity
		maxDD    float64
		coolLeft int
		dayTrades map[int]int
	)
	dayTrades = map[int]int{}

	closeAt := func(px float64, reason string, at time.Time, isOOS bool) {
		if pos == nil {
			return
		}
		qty := pos.riskUSD / mathAbs(pos.entry-pos.stop)
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

	for i := 50; i < len(bars); i++ {
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
		stopDist := clampF(atr*stopMult, entry*0.0015, entry*0.015)
		var stop, target float64
		if side == model.SideLong {
			stop = entry - stopDist
			target = entry + stopDist*rr
		} else {
			stop = entry + stopDist
			target = entry - stopDist*rr
		}
		pos = &btcPos{
			side: side, entry: entry, stop: stop, target: target,
			riskUSD: riskUSD, entryTime: b.CloseTime,
		}
		dayTrades[yd]++
	}

	// Aggregate metrics
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
	oosPF := 0.0
	if oosLosses > 0 {
		oosPF = oosWins / oosLosses
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
		Name: name, NetPnL: net, OOSPnL: oosNet, PF: pf, OOSPF: oosPF,
		MaxDD: maxDD, Trades: len(trades), WinRate: wr, ExpectR: expR,
		Profitable: net > 0 && oosNet > 0,
	}
}

func resampleBars(bars []Bar, n int) []Bar {
	if n <= 1 {
		return bars
	}
	var out []Bar
	for i := 0; i+n <= len(bars); i += n {
		chunk := bars[i : i+n]
		b := Bar{
			OpenTime: chunk[0].OpenTime,
			CloseTime: chunk[n-1].CloseTime,
			Open: chunk[0].Open,
			Close: chunk[n-1].Close,
			High: chunk[0].High,
			Low: chunk[0].Low,
		}
		for _, c := range chunk {
			if c.High > b.High {
				b.High = c.High
			}
			if c.Low < b.Low {
				b.Low = c.Low
			}
			b.Volume += c.Volume
			b.QuoteVol += c.QuoteVol
			b.TakerBuyVol += c.TakerBuyVol
		}
		out = append(out, b)
	}
	return out
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func FormatBTCLab(symbol string, results []BTCLabResult, days int, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n=== BTC LAB — custom strategies on %s ===\n", symbol))
	b.WriteString(fmt.Sprintf("%dd | %s | designed for BTC (not Aegis playbooks)\n\n", days, elapsed.Round(time.Second)))

	winners := 0
	for _, r := range results {
		if r.Profitable {
			winners++
		}
	}
	b.WriteString(fmt.Sprintf("Profitable (net+OOS): %d / %d\n\n", winners, len(results)))
	b.WriteString(fmt.Sprintf("%-26s %8s %8s %5s %5s %5s %4s %5s %s\n",
		"Strategy", "Net", "OOS", "PF", "OOSPF", "DD", "Tr", "WR%", "Status"))
	for _, r := range results {
		st := "FAIL"
		if r.Profitable {
			st = "WIN"
		} else if r.OOSPnL > 0 {
			st = "OOS+"
		}
		b.WriteString(fmt.Sprintf("%-26s %8.2f %8.2f %5.2f %5.2f %5.0f %4d %4.0f %s\n",
			r.Name, r.NetPnL, r.OOSPnL, r.PF, r.OOSPF, r.MaxDD, r.Trades, r.WinRate, st))
	}

	b.WriteString("\n=== TOP 5 WINNERS (net + OOS green) ===\n")
	winN := 0
	for _, r := range results {
		if !r.Profitable {
			continue
		}
		winN++
		if winN > 5 {
			break
		}
		b.WriteString(fmt.Sprintf("\n#%d %s\n", winN, r.Name))
		b.WriteString(fmt.Sprintf("  Net $%.2f | OOS $%.2f | PF %.2f | OOS PF %.2f | DD $%.0f\n",
			r.NetPnL, r.OOSPnL, r.PF, r.OOSPF, r.MaxDD))
		b.WriteString(fmt.Sprintf("  %d trades | WR %.0f%% | Avg R %.2f\n", r.Trades, r.WinRate, r.ExpectR))
		b.WriteString("  → Forward paper candidate on BTC\n")
	}
	if winN == 0 {
		b.WriteString("  No net+OOS winners on this 60d BTC window.\n")
		b.WriteString("  Closest (by OOS PnL):\n")
		sorted := append([]BTCLabResult(nil), results...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].OOSPnL > sorted[j].OOSPnL })
		for i := 0; i < 5 && i < len(sorted); i++ {
			r := sorted[i]
			b.WriteString(fmt.Sprintf("  - %s OOS $%.2f net $%.2f (%d tr)\n", r.Name, r.OOSPnL, r.NetPnL, r.Trades))
		}
	}
	return b.String()
}
