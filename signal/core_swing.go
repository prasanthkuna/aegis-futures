package signal

import (
	"time"

	"encore.app/config"
	"encore.app/market"
	"encore.app/model"
)

// EvalCoreSwing runs the symbol's validated playbook on 1h candles at the last bar.
func EvalCoreSwing(symbol string, h1 []market.Candle) (side model.Side, triggered bool, playbook string) {
	spec, ok := config.CoreSwingSpecFor(symbol)
	if !ok || len(h1) < 55 {
		return "", false, ""
	}
	i := len(h1) - 1
	switch spec.Playbook {
	case "S4_SQUEEZE_LIBERAL":
		side, triggered = evalS4SqueezeLiberal(h1, i)
	case "S11_EMA_TREND_STRICT":
		side, triggered = evalS11EMATrendStrict(h1, i)
	default:
		return "", false, ""
	}
	return side, triggered, spec.Playbook
}

func evalS4SqueezeLiberal(c []market.Candle, i int) (model.Side, bool) {
	if i < 35 {
		return "", false
	}
	atrNow := market.ATR(c[:i+1], 14)
	if atrNow <= 0 {
		return "", false
	}
	var atrs []float64
	for j := i - 19; j <= i; j++ {
		if j < 14 {
			continue
		}
		if a := market.ATR(c[:j+1], 14); a > 0 {
			atrs = append(atrs, a)
		}
	}
	if len(atrs) < 8 {
		return "", false
	}
	minATR := atrs[0]
	for _, a := range atrs {
		if a < minATR {
			minATR = a
		}
	}
	if atrNow > minATR*1.18 {
		return "", false
	}
	look := 10
	hi, lo := c[i-look].High, c[i-look].Low
	for j := i - look + 1; j < i; j++ {
		if c[j].High > hi {
			hi = c[j].High
		}
		if c[j].Low < lo {
			lo = c[j].Low
		}
	}
	px := c[i].Close
	e21 := market.EMA(c[:i+1], 21)
	if px > hi && e21 > 0 && px >= e21*0.998 {
		return model.SideLong, true
	}
	if px < lo && e21 > 0 && px <= e21*1.002 {
		return model.SideShort, true
	}
	return "", false
}

func evalS11EMATrendStrict(c []market.Candle, i int) (model.Side, bool) {
	if i < 55 {
		return "", false
	}
	slice := c[:i+1]
	px := c[i].Close
	e12 := market.EMA(slice, 12)
	e48 := market.EMA(slice, 48)
	rsi := market.RSI(slice, 14)
	if e12 <= 0 || e48 <= 0 {
		return "", false
	}
	if e12 > e48 && px >= e12*0.997 && rsi >= 40 && rsi <= 55 {
		return model.SideLong, true
	}
	if e12 < e48 && px <= e12*1.003 && rsi <= 60 && rsi >= 45 {
		return model.SideShort, true
	}
	return "", false
}

// BuildCoreSwingSignal builds a ProSignal for dashboard / entry.
func BuildCoreSwingSignal(symbol string, side model.Side, playbook string, st market.SymbolState, canTrade bool, block string) model.ProSignal {
	spec, _ := config.CoreSwingSpecFor(symbol)
	atr := market.ATR(st.Candles5m, 14)
	if len(st.Candles5m) >= 14 {
		h1, _, ok := marketResample1h(st.Candles5m)
		if ok && len(h1) >= 14 {
			atr = market.ATR(h1, 14)
		}
	}
	sig := model.ProSignal{
		Symbol: symbol, Side: side, Playbook: playbook,
		Strength: 100, Session: "CORE_1H", Price: st.LastPrice,
		SpreadBps: st.SpreadBps, QuoteVol24: st.QuoteVolume24h,
		IsCore: true, PlaybookTriggered: true,
		Tier: "CORE", WillFire: canTrade && side != "",
		CanExecute: canTrade && side != "", BlockReason: block,
		Extra: model.SignalExtra{
			ATR: atr, BtcRegime: "core_swing",
		},
	}
	_ = spec
	return sig
}

func marketResample1h(c5 []market.Candle) ([]market.Candle, time.Time, bool) {
	h1 := market.Resample5mTo1h(c5, config.CoreSwing1hBars)
	if len(h1) == 0 {
		return nil, time.Time{}, false
	}
	return h1, h1[len(h1)-1].CloseTime, true
}
