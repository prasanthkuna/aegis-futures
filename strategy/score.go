package strategy

import (
	"fmt"
	"time"

	"encore.app/config"
	"encore.app/market"
	"encore.app/model"
)

type Input struct {
	Symbol         string
	State          market.SymbolState
	CoinGlassScore float64
	BTCChange5mPct float64
}

type Result struct {
	TradeScore         float64
	VolumeComponent    float64
	CVDComponent       float64
	StructureComponent float64
	ContextComponent   float64
	DepthComponent     float64
	SessionComponent   float64
	Decision           string
	Reason             string
	SideHint           model.Side
}

func Evaluate(in Input) Result {
	r := Result{Decision: "skip", Reason: "no_setup"}

	volComp := volumeSurgeComponent(in.State.Candles5m)
	cvdComp, flow, cvdState := cvdComponent(in.State)
	structComp, brokeHigh, brokeLow := structureComponent(in.State.Candles5m)
	ctxComp := clamp01((in.CoinGlassScore + 1) / 2)
	depthComp := depthComponent(in.State.SpreadBps)
	sessComp := sessionScore(time.Now().UTC())

	r.VolumeComponent = volComp
	r.CVDComponent = cvdComp
	r.StructureComponent = structComp
	r.ContextComponent = ctxComp
	r.DepthComponent = depthComp
	r.SessionComponent = sessComp

	r.TradeScore = 0.25*volComp + 0.25*cvdComp + 0.20*structComp +
		0.15*ctxComp + 0.10*depthComp + 0.05*sessComp

	if r.TradeScore < config.MinTradeScore {
		r.Reason = fmt.Sprintf("score %.2f < %.2f", r.TradeScore, config.MinTradeScore)
		return r
	}

	side := model.SideLong
	if brokeLow && !brokeHigh {
		side = model.SideShort
	} else if brokeHigh {
		side = model.SideLong
	} else {
		r.Reason = "no_structure_break"
		return r
	}

	if side == model.SideLong {
		if in.BTCChange5mPct <= config.BTCBlockLongPct {
			r.Reason = "btc_dumping"
			return r
		}
		if flow != "buy" || cvdState != "up" {
			r.Reason = "taker_flow_not_confirmed"
			return r
		}
	} else {
		if in.BTCChange5mPct >= config.BTCBlockShortPct {
			r.Reason = "btc_pumping"
			return r
		}
		if flow != "sell" || cvdState != "down" {
			r.Reason = "taker_flow_not_confirmed"
			return r
		}
	}

	if in.State.SpreadBps > 12 {
		r.Reason = "spread_fail"
		return r
	}

	r.SideHint = side
	r.Decision = "trade"
	r.Reason = fmt.Sprintf("trade %s flow=%s score=%.2f", side, flow, r.TradeScore)
	return r
}

func volumeSurgeComponent(candles []market.Candle) float64 {
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

func cvdComponent(st market.SymbolState) (score float64, flow, state string) {
	buy, sell := st.TakerBuyVol, st.TakerSellVol
	total := buy + sell
	if total <= 0 {
		return 0, "flat", "flat"
	}
	delta := buy - sell
	if delta > 0 {
		flow, state = "buy", "up"
	} else if delta < 0 {
		flow, state = "sell", "down"
	} else {
		flow, state = "flat", "flat"
	}
	return clamp01(abs(delta) / total), flow, state
}

func structureComponent(candles []market.Candle) (score float64, brokeHigh, brokeLow bool) {
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
	brokeHigh = last.Close > hi && hi > 0
	brokeLow = last.Close < lo && lo > 0
	if brokeHigh || brokeLow {
		return 1, brokeHigh, brokeLow
	}
	return 0, false, false
}

func depthComponent(spreadBps float64) float64 {
	if spreadBps <= 3 {
		return 1
	}
	if spreadBps >= 15 {
		return 0
	}
	return clamp01(1 - (spreadBps-3)/12)
}

func sessionScore(now time.Time) float64 {
	h := now.Hour()
	switch {
	case h >= 0 && h < 8:
		return 0.6
	case h >= 8 && h < 14:
		return 0.85
	case h >= 14 && h < 22:
		return 1.0
	default:
		return 0.75
	}
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

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
