package signal

import (
	"math"

	"encore.app/config"
	"encore.app/market"
	"encore.app/model"
)

type PlaybookResult struct {
	ID        string
	Side      model.Side
	Score     float64 // 0-1 raw
	Triggered bool
}

func evalMomentum(st market.SymbolState, btcPct float64) PlaybookResult {
	vol := volumeSurge(st.Candles5m)
	cvd, flow, _ := cvdMetrics(st)
	_, brokeHi, brokeLo := structureBreak(st.Candles5m)
	depth := depthScore(st.SpreadBps)

	triggered := false
	side := model.SideLong
	switch {
	case brokeHi && flow != "sell":
		triggered, side = true, model.SideLong
	case brokeLo && flow != "buy":
		triggered, side = true, model.SideShort
	case vol >= 0.5 && flow == "buy" && cvd >= 0.35:
		triggered, side = true, model.SideLong
	case vol >= 0.5 && flow == "sell" && cvd >= 0.35:
		triggered, side = true, model.SideShort
	}
	if triggered {
		if side == model.SideLong && btcPct <= config.BTCBlockLongPct {
			triggered = false
		}
		if side == model.SideShort && btcPct >= config.BTCBlockShortPct {
			triggered = false
		}
	}
	raw := 0.30*vol + 0.30*cvd + 0.25*structureScore(brokeHi, brokeLo) + 0.15*depth
	return PlaybookResult{ID: "MOMENTUM_BURST", Side: side, Score: raw, Triggered: triggered}
}

func evalSessionBreakout(st market.SymbolState) PlaybookResult {
	hi, lo := market.SessionHighLow(st.Candles5m)
	last := lastClose(st.Candles5m)
	ema := market.EMA(st.Candles5m, 9)
	vol := volumeSurge(st.Candles5m)
	side := model.SideLong
	triggered := false
	if last > hi && hi > 0 && last > ema && ema > 0 {
		triggered, side = true, model.SideLong
	}
	if last < lo && lo > 0 && last < ema && ema > 0 {
		triggered, side = true, model.SideShort
	}
	emaSlope := 0.5
	if len(st.Candles5m) >= 3 {
		e1 := market.EMA(st.Candles5m[:len(st.Candles5m)-1], 9)
		if ema > e1 {
			emaSlope = 1
		} else if ema < e1 {
			emaSlope = 0
		}
	}
	raw := 0.4*structureScore(last > hi, last < lo) + 0.35*vol + 0.25*emaSlope
	return PlaybookResult{ID: "SESSION_BREAKOUT", Side: side, Score: raw, Triggered: triggered}
}

func evalForcedFlowFade(st market.SymbolState) PlaybookResult {
	vol := volumeSurge(st.Candles5m)
	vwap := market.VWAP(st.Candles5m)
	dev := market.VWAPDeviation(st.LastPrice, vwap)
	oiFlush := st.OIDeltaPct <= -2.5
	crowdedLong := st.FundingRate > 0.00025
	crowdedShort := st.FundingRate < -0.00025
	taker := st.TakerBuySellRatio
	side := model.SideLong
	triggered := false
	if oiFlush && dev <= -1.0 && (taker >= 1.1 || crowdedShort) {
		triggered, side = true, model.SideLong
	}
	if oiFlush && dev >= 1.0 && (taker <= 0.9 || crowdedLong) {
		triggered, side = true, model.SideShort
	}
	raw := clamp01(math.Abs(st.OIDeltaPct)/6)*0.35 + clamp01(math.Abs(dev)/2)*0.25 +
		vol*0.2 + clamp01(math.Abs(taker-1))*0.2
	return PlaybookResult{ID: "FORCED_FLOW_FADE", Side: side, Score: raw, Triggered: triggered}
}

// evalBBStretchRevert — Bandtastic / Strategy003 style: BB(20,2) stretch + RSI + flow confirm.
func evalBBStretchRevert(st market.SymbolState) PlaybookResult {
	candles := st.Candles5m
	last := lastClose(candles)
	lower, _, upper := market.BollingerBands(candles, 20, 2)
	rsi := market.RSI(candles, 14)
	cvd, flow, _ := cvdMetrics(st)
	vol := volumeSurge(candles)
	side := model.SideLong
	triggered := false
	if lower > 0 && last <= lower && rsi < 35 && (flow == "buy" || cvd >= 0.3) {
		triggered, side = true, model.SideLong
	}
	if upper > 0 && last >= upper && rsi > 65 && (flow == "sell" || cvd >= 0.3) {
		triggered, side = true, model.SideShort
	}
	dev := 0.0
	if lower > 0 && upper > 0 {
		mid := (lower + upper) / 2
		if mid > 0 {
			dev = math.Abs(last-mid) / mid * 100
		}
	}
	raw := clamp01(dev/3)*0.45 + clamp01((35-rsi)/35)*0.25 + cvd*0.2 + vol*0.1
	if rsi > 65 {
		raw = clamp01(dev/3)*0.45 + clamp01((rsi-65)/35)*0.25 + cvd*0.2 + vol*0.1
	}
	return PlaybookResult{ID: "BB_STRETCH_REVERT", Side: side, Score: raw, Triggered: triggered}
}

// evalVolClimaxFade — Strategy005 / v2 volume climax exhaustion at VWAP stretch.
func evalVolClimaxFade(st market.SymbolState) PlaybookResult {
	vol := volumeSurge(st.Candles5m)
	vwap := market.VWAP(st.Candles5m)
	dev := market.VWAPDeviation(st.LastPrice, vwap)
	cvd, flow, _ := cvdMetrics(st)
	rsi := market.RSI(st.Candles5m, 14)
	side := model.SideLong
	triggered := false
	if vol >= 0.65 && dev <= -0.9 && rsi < 42 && (flow == "buy" || cvd >= 0.25) {
		triggered, side = true, model.SideLong
	}
	if vol >= 0.65 && dev >= 0.9 && rsi > 58 && (flow == "sell" || cvd >= 0.25) {
		triggered, side = true, model.SideShort
	}
	raw := vol*0.35 + clamp01(math.Abs(dev)/2.5)*0.35 + cvd*0.2 + clamp01(math.Abs(rsi-50)/50)*0.1
	return PlaybookResult{ID: "VOL_CLIMAX_FADE", Side: side, Score: raw, Triggered: triggered}
}

func isChopRegime(st market.SymbolState) bool {
	candles := st.Candles5m
	if len(candles) < 25 || st.LastPrice <= 0 {
		return true
	}
	ema9 := market.EMA(candles, 9)
	ema21 := market.EMA(candles, 21)
	atr := market.ATR(candles, 14)
	trendSep := math.Abs(ema9-ema21) / st.LastPrice * 100
	atrPct := atr / st.LastPrice * 100
	return trendSep < 0.12 || atrPct < 0.35
}

func evalMeanRevert(st market.SymbolState, chop bool) PlaybookResult {
	vwap := market.VWAP(st.Candles5m)
	dev := market.VWAPDeviation(st.LastPrice, vwap)
	cvd, flow, _ := cvdMetrics(st)
	side := model.SideLong
	triggered := false
	if dev <= -1.2 && (flow == "buy" || cvd >= 0.3) {
		triggered, side = true, model.SideLong
	}
	if dev >= 1.2 && (flow == "sell" || cvd >= 0.3) {
		triggered, side = true, model.SideShort
	}
	if !chop && math.Abs(dev) < 2 {
		triggered = false
	}
	raw := clamp01(math.Abs(dev)/3)*0.5 + cvd*0.3 + volumeSurge(st.Candles5m)*0.2
	return PlaybookResult{ID: "MEAN_REVERT_VWAP", Side: side, Score: raw, Triggered: triggered}
}

func playbookAllowed(id string, sess SessionProfile) bool {
	for _, p := range sess.Playbooks {
		if p == id {
			return true
		}
	}
	return false
}

func volumeSurge(candles []market.Candle) float64 {
	if len(candles) < config.SwingLookback+1 {
		return 0
	}
	last := candles[len(candles)-1]
	var sum float64
	for i := len(candles) - config.SwingLookback - 1; i < len(candles)-1; i++ {
		sum += candles[i].Volume
	}
	avg := sum / float64(config.SwingLookback)
	if avg <= 0 {
		return 0
	}
	ratio := last.Volume / avg
	if ratio < config.VolumeSurgeMult {
		return clamp01(ratio / config.VolumeSurgeMult * 0.7)
	}
	return clamp01(0.7 + (ratio-config.VolumeSurgeMult)*0.15)
}

func cvdMetrics(st market.SymbolState) (score float64, flow, state string) {
	total := st.TakerBuyVol + st.TakerSellVol
	if total <= 0 {
		return 0, "flat", "flat"
	}
	delta := st.TakerBuyVol - st.TakerSellVol
	if delta > 0 {
		flow, state = "buy", "up"
	} else if delta < 0 {
		flow, state = "sell", "down"
	} else {
		flow, state = "flat", "flat"
	}
	return clamp01(math.Abs(delta) / total), flow, state
}

func structureBreak(candles []market.Candle) (score float64, brokeHi, brokeLo bool) {
	if len(candles) < config.SwingLookback+1 {
		return 0, false, false
	}
	last := candles[len(candles)-1]
	var hi, lo float64
	for i := len(candles) - config.SwingLookback - 1; i < len(candles)-1; i++ {
		if candles[i].High > hi {
			hi = candles[i].High
		}
		if lo == 0 || candles[i].Low < lo {
			lo = candles[i].Low
		}
	}
	brokeHi = last.Close > hi && hi > 0
	brokeLo = last.Close < lo && lo > 0
	return structureScore(brokeHi, brokeLo), brokeHi, brokeLo
}

func structureScore(brokeHi, brokeLo bool) float64 {
	if brokeHi || brokeLo {
		return 1
	}
	return 0
}

func depthScore(spreadBps float64) float64 {
	if spreadBps <= 3 {
		return 1
	}
	if spreadBps >= 15 {
		return 0
	}
	return clamp01(1 - (spreadBps-3)/12)
}

func lastClose(candles []market.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	return candles[len(candles)-1].Close
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
