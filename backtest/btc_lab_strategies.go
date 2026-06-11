package backtest

import (
	"math"
	"time"

	"encore.app/market"
	"encore.app/model"
)

// BTCStrategyFunc emits a directional entry at bar index i (closed bar).
type BTCStrategyFunc func(bars []Bar, i int) (side model.Side, ok bool)

func barsToCandles(bars []Bar, upto int) []market.Candle {
	out := make([]market.Candle, upto+1)
	for j := 0; j <= upto; j++ {
		out[j] = bars[j].Candle()
	}
	return out
}

// S1_EMA_PULLBACK — trend pullback: EMA50 bias + RSI reset in trend direction.
func S1_EMA_PULLBACK(bars []Bar, i int) (model.Side, bool) {
	if i < 55 {
		return "", false
	}
	c := barsToCandles(bars, i)
	px := bars[i].Close
	ema50 := market.EMA(c, 50)
	ema9 := market.EMA(c, 9)
	rsi := market.RSI(c, 14)
	prev := barsToCandles(bars, i-1)
	rsiPrev := market.RSI(prev, 14)
	atr := market.ATR(c, 14)
	if atr <= 0 || ema50 <= 0 {
		return "", false
	}
	// Long: uptrend, price dipped toward EMA9, RSI recovering from 35-48
	if px > ema50 && px >= ema9*0.998 && rsiPrev < 48 && rsi > rsiPrev && rsi >= 38 && rsi <= 58 {
		return model.SideLong, true
	}
	// Short: downtrend mirror
	if px < ema50 && px <= ema9*1.002 && rsiPrev > 52 && rsi < rsiPrev && rsi <= 62 && rsi >= 42 {
		return model.SideShort, true
	}
	return "", false
}

// S2_BB_RANGE_FADE — low ATR% range: fade Bollinger 2σ with RSI extreme.
func S2_BB_RANGE_FADE(bars []Bar, i int) (model.Side, bool) {
	if i < 30 {
		return "", false
	}
	c := barsToCandles(bars, i)
	px := bars[i].Close
	lower, _, upper := market.BollingerBands(c, 20, 2)
	rsi := market.RSI(c, 14)
	atr := market.ATR(c, 14)
	if atr <= 0 || px <= 0 {
		return "", false
	}
	atrPct := atr / px * 100
	if atrPct > 0.45 { // range regime only
		return "", false
	}
	if lower > 0 && px <= lower && rsi < 32 {
		return model.SideLong, true
	}
	if upper > 0 && px >= upper && rsi > 68 {
		return model.SideShort, true
	}
	return "", false
}

// S3_LONDON_BREAK — break Asia session box (00-08 UTC) during London 08-12 UTC.
func S3_LONDON_BREAK(bars []Bar, i int) (model.Side, bool) {
	if i < 120 {
		return "", false
	}
	t := bars[i].CloseTime.UTC()
	h := t.Hour()
	if h < 8 || h >= 12 {
		return "", false
	}
	// Asia box: same UTC day bars with hour < 8
	var asiaHi, asiaLo float64
	day := t.YearDay()
	for j := i; j >= 0 && j >= i-200; j-- {
		bj := bars[j].CloseTime.UTC()
		if bj.YearDay() != day {
			break
		}
		if bj.Hour() >= 8 {
			continue
		}
		if bars[j].High > asiaHi {
			asiaHi = bars[j].High
		}
		if asiaLo == 0 || bars[j].Low < asiaLo {
			asiaLo = bars[j].Low
		}
	}
	if asiaHi <= 0 || asiaLo <= 0 || asiaHi <= asiaLo {
		return "", false
	}
	box := (asiaHi - asiaLo) / asiaLo * 100
	if box < 0.15 || box > 2.5 {
		return "", false
	}
	px := bars[i].Close
	prev := bars[i-1].Close
	if prev <= asiaHi && px > asiaHi {
		return model.SideLong, true
	}
	if prev >= asiaLo && px < asiaLo {
		return model.SideShort, true
	}
	return "", false
}

// S3_LONDON_STRICT — tighter Asia box + volume confirm on break + London 08-10 only.
func S3_LONDON_STRICT(bars []Bar, i int) (model.Side, bool) {
	if i < 120 {
		return "", false
	}
	t := bars[i].CloseTime.UTC()
	h := t.Hour()
	if h < 8 || h >= 10 {
		return "", false
	}
	wd := t.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return "", false
	}
	var asiaHi, asiaLo float64
	day := t.YearDay()
	for j := i; j >= 0 && j >= i-200; j-- {
		bj := bars[j].CloseTime.UTC()
		if bj.YearDay() != day {
			break
		}
		if bj.Hour() >= 8 {
			continue
		}
		if bars[j].High > asiaHi {
			asiaHi = bars[j].High
		}
		if asiaLo == 0 || bars[j].Low < asiaLo {
			asiaLo = bars[j].Low
		}
	}
	if asiaHi <= 0 || asiaLo <= 0 {
		return "", false
	}
	box := (asiaHi - asiaLo) / asiaLo * 100
	if box < 0.2 || box > 1.8 {
		return "", false
	}
	var volAvg float64
	for j := i - 20; j < i; j++ {
		volAvg += bars[j].Volume
	}
	volAvg /= 20
	if volAvg <= 0 || bars[i].Volume < volAvg*1.3 {
		return "", false
	}
	px := bars[i].Close
	prev := bars[i-1].Close
	if prev <= asiaHi && px > asiaHi {
		return model.SideLong, true
	}
	if prev >= asiaLo && px < asiaLo {
		return model.SideShort, true
	}
	return "", false
}

// S4_SQUEEZE_BREAK — ATR compression then break of 12-bar high/low.
func S4_SQUEEZE_BREAK(bars []Bar, i int) (model.Side, bool) {
	if i < 40 {
		return "", false
	}
	c := barsToCandles(bars, i)
	atrNow := market.ATR(c, 14)
	if atrNow <= 0 {
		return "", false
	}
	// ATR vs 20-bar ATR median — compression
	var atrs []float64
	for j := i - 19; j <= i; j++ {
		cc := barsToCandles(bars, j)
		a := market.ATR(cc, 14)
		if a > 0 {
			atrs = append(atrs, a)
		}
	}
	if len(atrs) < 10 {
		return "", false
	}
	minATR := atrs[0]
	for _, a := range atrs {
		if a < minATR {
			minATR = a
		}
	}
	if atrNow > minATR*1.08 {
		return "", false
	}
	look := 12
	hi, lo := bars[i-look].High, bars[i-look].Low
	for j := i - look + 1; j < i; j++ {
		if bars[j].High > hi {
			hi = bars[j].High
		}
		if bars[j].Low < lo {
			lo = bars[j].Low
		}
	}
	px := bars[i].Close
	ema21 := market.EMA(c, 21)
	if px > hi && ema21 > 0 && px > ema21 {
		return model.SideLong, true
	}
	if px < lo && ema21 > 0 && px < ema21 {
		return model.SideShort, true
	}
	return "", false
}

// S4_SQUEEZE_LIBERAL — looser compression + shorter channel for more 1h entries.
func S4_SQUEEZE_LIBERAL(bars []Bar, i int) (model.Side, bool) {
	if i < 35 {
		return "", false
	}
	c := barsToCandles(bars, i)
	atrNow := market.ATR(c, 14)
	if atrNow <= 0 {
		return "", false
	}
	var atrs []float64
	for j := i - 19; j <= i; j++ {
		cc := barsToCandles(bars, j)
		if a := market.ATR(cc, 14); a > 0 {
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
	hi, lo := bars[i-look].High, bars[i-look].Low
	for j := i - look + 1; j < i; j++ {
		if bars[j].High > hi {
			hi = bars[j].High
		}
		if bars[j].Low < lo {
			lo = bars[j].Low
		}
	}
	px := bars[i].Close
	ema21 := market.EMA(c, 21)
	if px > hi && ema21 > 0 && px >= ema21*0.998 {
		return model.SideLong, true
	}
	if px < lo && ema21 > 0 && px <= ema21*1.002 {
		return model.SideShort, true
	}
	return "", false
}

// S14_DONCHIAN_LIBERAL — lower volume gate + 16-bar channel.
func S14_DONCHIAN_LIBERAL(bars []Bar, i int) (model.Side, bool) {
	if i < 22 {
		return "", false
	}
	look := 16
	hi, lo := bars[i-look].High, bars[i-look].Low
	for j := i - look + 1; j < i; j++ {
		if bars[j].High > hi {
			hi = bars[j].High
		}
		if bars[j].Low < lo {
			lo = bars[j].Low
		}
	}
	var volAvg float64
	for j := i - 16; j < i; j++ {
		volAvg += bars[j].Volume
	}
	volAvg /= 16
	if volAvg <= 0 || bars[i].Volume < volAvg*1.15 {
		return "", false
	}
	px := bars[i].Close
	c := barsToCandles(bars, i)
	e48 := market.EMA(c, 48)
	if e48 <= 0 {
		return "", false
	}
	if px > hi && px >= e48*0.997 {
		return model.SideLong, true
	}
	if px < lo && px <= e48*1.003 {
		return model.SideShort, true
	}
	return "", false
}

// S11_EMA_TREND_LIBERAL — wider pullback + RSI band for more trend entries.
func S11_EMA_TREND_LIBERAL(bars []Bar, i int) (model.Side, bool) {
	if i < 50 {
		return "", false
	}
	c := barsToCandles(bars, i)
	px := bars[i].Close
	e12 := market.EMA(c, 12)
	e48 := market.EMA(c, 48)
	rsi := market.RSI(c, 14)
	if e12 <= 0 || e48 <= 0 {
		return "", false
	}
	if e12 > e48 && px >= e12*0.994 && rsi >= 35 && rsi <= 62 {
		return model.SideLong, true
	}
	if e12 < e48 && px <= e12*1.006 && rsi <= 65 && rsi >= 38 {
		return model.SideShort, true
	}
	return "", false
}

// S5_RSI2_SNAPBACK — short-term RSI(2) exhaustion snap (Connors-style).
func S5_RSI2_SNAPBACK(bars []Bar, i int) (model.Side, bool) {
	if i < 20 {
		return "", false
	}
	c := barsToCandles(bars, i)
	r2 := market.RSI(c, 2)
	prev := barsToCandles(bars, i-1)
	r2p := market.RSI(prev, 2)
	atr := market.ATR(c, 14)
	px := bars[i].Close
	if atr <= 0 || px <= 0 {
		return "", false
	}
	// Avoid dead market
	if atr/px*100 < 0.08 {
		return "", false
	}
	if r2p <= 8 && r2 > r2p+5 {
		return model.SideLong, true
	}
	if r2p >= 92 && r2 < r2p-5 {
		return model.SideShort, true
	}
	return "", false
}

// S6_VWAP_TOUCH — session VWAP touch fade with taker imbalance.
func S6_VWAP_TOUCH(bars []Bar, i int) (model.Side, bool) {
	if i < 30 {
		return "", false
	}
	// Session VWAP from UTC midnight
	day := bars[i].CloseTime.UTC().YearDay()
	var pv, vol float64
	for j := i; j >= 0; j-- {
		if bars[j].CloseTime.UTC().YearDay() != day {
			break
		}
		tp := (bars[j].High + bars[j].Low + bars[j].Close) / 3
		pv += tp * bars[j].Volume
		vol += bars[j].Volume
	}
	if vol <= 0 {
		return "", false
	}
	vwap := pv / vol
	px := bars[i].Close
	dev := (px - vwap) / vwap * 100
	buy := bars[i].TakerBuyVol
	sell := bars[i].TakerSellVol()
	total := buy + sell
	if total <= 0 {
		return "", false
	}
	delta := (buy - sell) / total
	c := barsToCandles(bars, i)
	rsi := market.RSI(c, 14)
	if dev <= -0.35 && delta > 0.05 && rsi < 45 {
		return model.SideLong, true
	}
	if dev >= 0.35 && delta < -0.05 && rsi > 55 {
		return model.SideShort, true
	}
	return "", false
}

// S7_MOMENTUM_IGNITION — volume spike + close in top/bottom 25% of bar range.
func S7_MOMENTUM_IGNITION(bars []Bar, i int) (model.Side, bool) {
	if i < 25 {
		return "", false
	}
	var volSum float64
	for j := i - 20; j < i; j++ {
		volSum += bars[j].Volume
	}
	avg := volSum / 20
	if avg <= 0 {
		return "", false
	}
	if bars[i].Volume < avg*2.2 {
		return "", false
	}
	b := bars[i]
	rng := b.High - b.Low
	if rng <= 0 {
		return "", false
	}
	pos := (b.Close - b.Low) / rng
	c := barsToCandles(bars, i)
	ema9 := market.EMA(c, 9)
	ema21 := market.EMA(c, 21)
	if pos >= 0.75 && ema9 > ema21 {
		return model.SideLong, true
	}
	if pos <= 0.25 && ema9 < ema21 {
		return model.SideShort, true
	}
	return "", false
}

// S8_LONG_LONDON — long-only Asia box break (BTC drift bias).
func S8_LONG_LONDON(bars []Bar, i int) (model.Side, bool) {
	side, ok := S3_LONDON_STRICT(bars, i)
	if ok && side == model.SideLong {
		return model.SideLong, true
	}
	return "", false
}

// S9_DEEP_BB — fade only 2.5σ BB in compression (stricter range fade).
func S9_DEEP_BB(bars []Bar, i int) (model.Side, bool) {
	if i < 30 {
		return "", false
	}
	c := barsToCandles(bars, i)
	px := bars[i].Close
	_, _, upper := market.BollingerBands(c, 20, 2.5)
	lower, _, _ := market.BollingerBands(c, 20, 2.5)
	// recompute lower/upper with 2.5 std - fix call
	lower, _, upper = bollinger(c, 20, 2.5)
	rsi := market.RSI(c, 14)
	atr := market.ATR(c, 14)
	if atr <= 0 || px <= 0 || atr/px*100 > 0.35 {
		return "", false
	}
	if lower > 0 && px <= lower && rsi < 28 {
		return model.SideLong, true
	}
	if upper > 0 && px >= upper && rsi > 72 {
		return model.SideShort, true
	}
	return "", false
}

func bollinger(c []market.Candle, period int, stds float64) (lower, mid, upper float64) {
	return market.BollingerBands(c, period, stds)
}

// S10_H1_INSIDE_BREAK — prior bar inside bar, break mother bar high/low.
func S10_H1_INSIDE_BREAK(bars []Bar, i int) (model.Side, bool) {
	if i < 30 {
		return "", false
	}
	mom := bars[i-1]
	inside := bars[i-2]
	if inside.High >= mom.High || inside.Low <= mom.Low {
		return "", false
	}
	px := bars[i].Close
	if px > mom.High {
		return model.SideLong, true
	}
	if px < mom.Low {
		return model.SideShort, true
	}
	return "", false
}

// S11_H1_EMA_TREND — EMA12/48 alignment + pullback to EMA12.
func S11_H1_EMA_TREND(bars []Bar, i int) (model.Side, bool) {
	if i < 55 {
		return "", false
	}
	c := barsToCandles(bars, i)
	px := bars[i].Close
	e12 := market.EMA(c, 12)
	e48 := market.EMA(c, 48)
	rsi := market.RSI(c, 14)
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

// S13_NY_CONTINUATION — London sets bias; NY session (13-17 UTC) continuation with EMA21.
func S13_NY_CONTINUATION(bars []Bar, i int) (model.Side, bool) {
	if i < 60 {
		return "", false
	}
	t := bars[i].CloseTime.UTC()
	h := t.Hour()
	if h < 13 || h >= 17 {
		return "", false
	}
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return "", false
	}
	day := t.YearDay()
	var londonHi, londonLo float64
	for j := i; j >= 0 && j >= i-30; j-- {
		bj := bars[j].CloseTime.UTC()
		if bj.YearDay() != day {
			break
		}
		hh := bj.Hour()
		if hh < 8 || hh >= 12 {
			continue
		}
		if bars[j].High > londonHi {
			londonHi = bars[j].High
		}
		if londonLo == 0 || bars[j].Low < londonLo {
			londonLo = bars[j].Low
		}
	}
	if londonHi <= 0 || londonLo <= 0 {
		return "", false
	}
	c := barsToCandles(bars, i)
	px := bars[i].Close
	e21 := market.EMA(c, 21)
	if e21 <= 0 {
		return "", false
	}
	mid := (londonHi + londonLo) / 2
	if px > mid && px > e21 && bars[i-1].Close <= londonHi && px > londonHi*0.999 {
		return model.SideLong, true
	}
	if px < mid && px < e21 && bars[i-1].Close >= londonLo && px < londonLo*1.001 {
		return model.SideShort, true
	}
	return "", false
}

// S14_DONCHIAN_VOL — 20-bar channel break with volume surge.
func S14_DONCHIAN_VOL(bars []Bar, i int) (model.Side, bool) {
	if i < 25 {
		return "", false
	}
	look := 20
	hi, lo := bars[i-look].High, bars[i-look].Low
	for j := i - look + 1; j < i; j++ {
		if bars[j].High > hi {
			hi = bars[j].High
		}
		if bars[j].Low < lo {
			lo = bars[j].Low
		}
	}
	var volAvg float64
	for j := i - 20; j < i; j++ {
		volAvg += bars[j].Volume
	}
	volAvg /= 20
	if volAvg <= 0 || bars[i].Volume < volAvg*1.4 {
		return "", false
	}
	px := bars[i].Close
	c := barsToCandles(bars, i)
	e48 := market.EMA(c, 48)
	if e48 <= 0 {
		return "", false
	}
	if px > hi && px > e48 {
		return model.SideLong, true
	}
	if px < lo && px < e48 {
		return model.SideShort, true
	}
	return "", false
}

// S15_ASIA_RANGE_FADE — fade Asia box extremes at London open (07-09 UTC).
func S15_ASIA_RANGE_FADE(bars []Bar, i int) (model.Side, bool) {
	if i < 100 {
		return "", false
	}
	t := bars[i].CloseTime.UTC()
	h := t.Hour()
	if h < 7 || h >= 9 {
		return "", false
	}
	day := t.YearDay()
	var asiaHi, asiaLo float64
	for j := i; j >= 0 && j >= i-200; j-- {
		bj := bars[j].CloseTime.UTC()
		if bj.YearDay() != day {
			break
		}
		if bj.Hour() >= 7 {
			continue
		}
		if bars[j].High > asiaHi {
			asiaHi = bars[j].High
		}
		if asiaLo == 0 || bars[j].Low < asiaLo {
			asiaLo = bars[j].Low
		}
	}
	if asiaHi <= 0 || asiaLo <= 0 {
		return "", false
	}
	box := (asiaHi - asiaLo) / asiaLo * 100
	if box < 0.25 || box > 1.5 {
		return "", false
	}
	px := bars[i].Close
	c := barsToCandles(bars, i)
	rsi := market.RSI(c, 14)
	if px >= asiaHi*0.9995 && rsi > 62 {
		return model.SideShort, true
	}
	if px <= asiaLo*1.0005 && rsi < 38 {
		return model.SideLong, true
	}
	return "", false
}

// S12_H1_DOJI_FADE — doji after stretch from EMA48.
func S12_H1_DOJI_FADE(bars []Bar, i int) (model.Side, bool) {
	if i < 50 {
		return "", false
	}
	b := bars[i]
	body := math.Abs(b.Close - b.Open)
	rng := b.High - b.Low
	if rng <= 0 || body/rng > 0.25 {
		return "", false
	}
	c := barsToCandles(bars, i)
	e48 := market.EMA(c, 48)
	if e48 <= 0 {
		return "", false
	}
	dev := (b.Close - e48) / e48 * 100
	if dev <= -1.2 {
		return model.SideLong, true
	}
	if dev >= 1.2 {
		return model.SideShort, true
	}
	return "", false
}

func allBTCLabStrategies() map[string]BTCStrategyFunc {
	return map[string]BTCStrategyFunc{
		"S1_EMA_PULLBACK":      S1_EMA_PULLBACK,
		"S2_BB_RANGE_FADE":     S2_BB_RANGE_FADE,
		"S3_LONDON_BREAK":      S3_LONDON_BREAK,
		"S3_LONDON_STRICT":     S3_LONDON_STRICT,
		"S4_SQUEEZE_BREAK":     S4_SQUEEZE_BREAK,
		"S5_RSI2_SNAPBACK":     S5_RSI2_SNAPBACK,
		"S6_VWAP_TOUCH":        S6_VWAP_TOUCH,
		"S7_MOMENTUM_IGNITION": S7_MOMENTUM_IGNITION,
		"S8_LONG_LONDON":       S8_LONG_LONDON,
		"S9_DEEP_BB":           S9_DEEP_BB,
		"S10_H1_INSIDE_BREAK":  S10_H1_INSIDE_BREAK,
		"S11_H1_EMA_TREND":     S11_H1_EMA_TREND,
		"S12_H1_DOJI_FADE":     S12_H1_DOJI_FADE,
		"S13_NY_CONTINUATION":  S13_NY_CONTINUATION,
		"S14_DONCHIAN_VOL":     S14_DONCHIAN_VOL,
		"S15_ASIA_RANGE_FADE":  S15_ASIA_RANGE_FADE,
	}
}

func barRangePct(bars []Bar, i, n int) float64 {
	if i < n {
		return 0
	}
	hi, lo := bars[i].High, bars[i].Low
	for j := i - n + 1; j <= i; j++ {
		if bars[j].High > hi {
			hi = bars[j].High
		}
		if bars[j].Low < lo {
			lo = bars[j].Low
		}
	}
	mid := (hi + lo) / 2
	if mid <= 0 {
		return 0
	}
	return (hi - lo) / mid * 100
}

func clampF(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
