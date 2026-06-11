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

// BTCProdRow is one liberal 1h prod config result.
type BTCProdRow struct {
	Name       string
	Variant    string
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

func prodBTCStrategies() []struct {
	name    string
	variant string
	fn      BTCStrategyFunc
} {
	return []struct {
		name    string
		variant string
		fn      BTCStrategyFunc
	}{
		{"S4_SQUEEZE", "strict", S4_SQUEEZE_BREAK},
		{"S4_SQUEEZE", "liberal", S4_SQUEEZE_LIBERAL},
		{"S14_DONCHIAN", "strict", S14_DONCHIAN_VOL},
		{"S14_DONCHIAN", "liberal", S14_DONCHIAN_LIBERAL},
		{"S11_EMA_TREND", "strict", S11_H1_EMA_TREND},
		{"S11_EMA_TREND", "liberal", S11_EMA_TREND_LIBERAL},
	}
}

// RunBTCProdLiberal tests proven 1h strategies with loose trade caps and liberal entries.
func RunBTCProdLiberal(ctx context.Context, r *Runner, symbol string, days int) ([]BTCProdRow, BTCProdRow, time.Duration, error) {
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
		return nil, BTCProdRow{}, 0, err
	}
	ds, err = FilterDataset(ds, []string{symbol})
	if err != nil {
		return nil, BTCProdRow{}, 0, err
	}
	bars := resampleBars(ds.All[symbol], config.BTCProdTimeframeBars)
	if len(bars) < 100 {
		return nil, BTCProdRow{}, 0, fmt.Errorf("insufficient 1h bars for %s", symbol)
	}
	oosIdx := int(float64(len(bars)) * 0.7)
	fmt.Printf("BTC prod liberal 1h: %d bars | risk $%.0f | %dx | max %d/day\n",
		len(bars), config.BTCProdRiskUSD, config.BTCProdMaxLeverage, config.BTCProdMaxTradesPerDay)

	sz := btcSizing{
		riskUSD:       config.BTCProdRiskUSD,
		activeCapital: config.BTCProdActiveCapital,
		leverage:      config.BTCProdMaxLeverage,
	}
	parBase := btcLabParams{maxHold: 36, cooldown: 2}

	var rows []BTCProdRow
	for _, st := range prodBTCStrategies() {
		var best BTCProdRow
		for _, maxDay := range []int{4, 5, 6} {
			for _, rr := range []float64{2.5, 3.0, 3.5, 4.0} {
				for _, stop := range []float64{1.0, 1.2, 1.4, 1.8} {
					par := parBase
					par.rr = rr
					par.stopATR = stop
					par.maxPerDay = maxDay
					res := runOneBTCStrategySized(st.name+"_"+st.variant, st.fn, bars, oosIdx, par, sz)
					if res.Trades < 5 {
						continue
					}
					row := BTCProdRow{
						Name: st.name, Variant: st.variant,
						RiskUSD: sz.riskUSD, Leverage: sz.leverage, MaxPerDay: maxDay,
						RR: rr, StopATR: stop,
						NetPnL: res.NetPnL, OOSPnL: res.OOSPnL, PF: res.PF,
						MaxDD: res.MaxDD, Trades: res.Trades, WinRate: res.WinRate,
						ExpectR: res.ExpectR, Profitable: res.Profitable,
					}
					if row.Profitable && (best.Trades == 0 || row.NetPnL > best.NetPnL) {
						best = row
					}
					if !row.Profitable && best.Trades == 0 && row.NetPnL > best.NetPnL {
						best = row
					}
				}
			}
		}
		if best.Trades > 0 {
			rows = append(rows, best)
			st := "FAIL"
			if best.Profitable {
				st = "WIN"
			}
			fmt.Printf("  %s %s net=%.2f oos=%.2f tr=%d mpd=%d rr=%.1f %s\n",
				best.Name, best.Variant, best.NetPnL, best.OOSPnL, best.Trades,
				best.MaxPerDay, best.RR, st)
		}
	}

	combo := runBTCProdCombo(bars, oosIdx, sz)
	fmt.Printf("  COMBO_BOOK net=%.2f oos=%.2f tr=%d mpd=6\n", combo.NetPnL, combo.OOSPnL, combo.Trades)

	sort.Slice(rows, func(i, j int) bool { return rows[i].NetPnL > rows[j].NetPnL })
	return rows, combo, time.Since(start), nil
}

// runBTCProdCombo runs liberal S4+S14+S11 with shared position slot, max 6 trades/day.
func runBTCProdCombo(bars []Bar, oosIdx int, sz btcSizing) BTCProdRow {
	type leg struct {
		name string
		fn   BTCStrategyFunc
		par  btcLabParams
	}
	// Best liberal params from prior refine (starting point; combo uses fixed liberal rules).
	legs := []leg{
		{"S4_LIB", S4_SQUEEZE_LIBERAL, btcLabParams{stopATR: 1.4, rr: 3.5, maxHold: 36, cooldown: 2, maxPerDay: 6}},
		{"S14_LIB", S14_DONCHIAN_LIBERAL, btcLabParams{stopATR: 1.8, rr: 3.5, maxHold: 36, cooldown: 2, maxPerDay: 6}},
		{"S11_LIB", S11_EMA_TREND_LIBERAL, btcLabParams{stopATR: 1.0, rr: 4.0, maxHold: 36, cooldown: 2, maxPerDay: 6}},
	}

	var (
		pos       *btcPos
		trades    []Trade
		equity    = sz.activeCapital
		peak      = equity
		maxDD     float64
		coolLeft  int
		dayTrades = map[int]int{}
		par       = btcLabParams{maxHold: 36, cooldown: 2, maxPerDay: 6}
	)

	closeAt := func(px float64, reason, playbook string, at time.Time, isOOS bool) {
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
			Symbol: "BTCUSDT", Side: pos.side, Playbook: playbook,
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
			legPar := legs[0].par
			for _, lg := range legs {
				if lg.name == pos.playbook {
					legPar = lg.par
					break
				}
			}
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
				closeAt(px, "stop", pos.playbook, b.CloseTime, isOOS)
				continue
			}
			if hitTP {
				closeAt(pos.target, "target", pos.playbook, b.CloseTime, isOOS)
				continue
			}
			if pos.barsHeld >= legPar.maxHold {
				closeAt(b.Close, "time", pos.playbook, b.CloseTime, isOOS)
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

		var fired *leg
		for li := range legs {
			lg := &legs[li]
			side, ok := lg.fn(bars, i)
			if !ok {
				continue
			}
			fired = lg
			c := barsToCandles(bars, i)
			atr := market.ATR(c, 14)
			if atr <= 0 {
				fired = nil
				continue
			}
			entry := b.Close
			stopDist := clampF(atr*lg.par.stopATR, entry*0.0015, entry*0.025)
			var stop, target float64
			if side == model.SideLong {
				stop = entry - stopDist
				target = entry + stopDist*lg.par.rr
			} else {
				stop = entry + stopDist
				target = entry - stopDist*lg.par.rr
			}
			qty := execution.RiskQuantity(sz.activeCapital, sz.riskUSD, entry, stop, sz.leverage)
			pos = &btcPos{
				side: side, entry: entry, stop: stop, target: target,
				riskUSD: sz.riskUSD, entryTime: b.CloseTime, qty: qty,
				playbook: lg.name,
			}
			dayTrades[yd]++
			break
		}
		_ = fired
	}

	var net, oosNet, wins, losses, sumR float64
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
	return BTCProdRow{
		Name: "COMBO_LIBERAL_BOOK", Variant: "S4+S14+S11",
		RiskUSD: sz.riskUSD, Leverage: sz.leverage, MaxPerDay: 6,
		NetPnL: net, OOSPnL: oosNet, PF: pf, MaxDD: maxDD,
		Trades: len(trades), WinRate: wr, ExpectR: expR,
		Profitable: net > 0 && oosNet > 0,
	}
}

func FormatBTCProd(symbol string, rows []BTCProdRow, combo BTCProdRow, days int, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n=== BTC PROD LIBERAL — 1h swing on %s ===\n", symbol))
	b.WriteString(fmt.Sprintf("%dd | %s | $%.0f risk | %dx lev | max 4-6 trades/day\n\n",
		days, elapsed.Round(time.Second), config.BTCProdRiskUSD, config.BTCProdMaxLeverage))

	winN := 0
	b.WriteString("--- Per strategy (best liberal grid) ---\n")
	b.WriteString(fmt.Sprintf("%-14s %-7s %6s %6s %4s %4s %5s %5s %4s\n",
		"Strategy", "Variant", "Net", "OOS", "Tr", "mpd", "RR", "Stop", "OK"))
	for _, r := range rows {
		ok := "no"
		if r.Profitable {
			ok = "YES"
			winN++
		}
		b.WriteString(fmt.Sprintf("%-14s %-7s %6.2f %6.2f %4d %4d %5.1f %5.1f %4s\n",
			r.Name, r.Variant, r.NetPnL, r.OOSPnL, r.Trades, r.MaxPerDay, r.RR, r.StopATR, ok))
	}

	b.WriteString("\n--- Combined book (liberal S4+S14+S11, shared slot, max 6/day) ---\n")
	st := "FAIL"
	if combo.Profitable {
		st = "WIN"
	}
	b.WriteString(fmt.Sprintf("COMBO net $%.2f | OOS $%.2f | PF %.2f | DD $%.0f | %d tr WR %.0f%% [%s]\n",
		combo.NetPnL, combo.OOSPnL, combo.PF, combo.MaxDD, combo.Trades, combo.WinRate, st))

	b.WriteString("\n--- vs prior strict prod (max 1-2/day) ---\n")
	b.WriteString("Prior best: S4 $84 net (17 tr, max 1/day) | S14 $84 net (34 tr)\n")
	b.WriteString("Liberal goal: more trades/day → higher total $ if PF holds\n")

	if len(rows) > 0 {
		b.WriteString("\n--- Recommended prod config ---\n")
		top := rows[0]
		for _, r := range rows {
			if r.Profitable && r.NetPnL > top.NetPnL {
				top = r
			}
		}
		b.WriteString(fmt.Sprintf("Top single: %s (%s) rr%.1f stop%.1f max%d/day → $%.2f net\n",
			top.Name, top.Variant, top.RR, top.StopATR, top.MaxPerDay, top.NetPnL))
		if combo.Profitable && combo.NetPnL > top.NetPnL {
			b.WriteString("Use COMBO_BOOK — beats best single strategy on net $\n")
		}
	}
	return b.String()
}
