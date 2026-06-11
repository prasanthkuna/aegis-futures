package market

import "math"

func VWAP(candles []Candle) float64 {
	var pv, vol float64
	for _, c := range candles {
		typ := (c.High + c.Low + c.Close) / 3
		pv += typ * c.Volume
		vol += c.Volume
	}
	if vol <= 0 {
		return 0
	}
	return pv / vol
}

func EMA(candles []Candle, period int) float64 {
	if len(candles) < period || period <= 0 {
		return 0
	}
	k := 2.0 / float64(period+1)
	ema := candles[len(candles)-period].Close
	for i := len(candles) - period + 1; i < len(candles); i++ {
		ema = candles[i].Close*k + ema*(1-k)
	}
	return ema
}

func ATR(candles []Candle, period int) float64 {
	if len(candles) < period+1 || period <= 0 {
		return 0
	}
	var sum float64
	for i := len(candles) - period; i < len(candles); i++ {
		tr := candles[i].High - candles[i].Low
		if i > 0 {
			prevClose := candles[i-1].Close
			tr = math.Max(tr, math.Abs(candles[i].High-prevClose))
			tr = math.Max(tr, math.Abs(candles[i].Low-prevClose))
		}
		sum += tr
	}
	return sum / float64(period)
}

func SessionHighLow(candles []Candle) (hi, lo float64) {
	for _, c := range candles {
		if c.High > hi {
			hi = c.High
		}
		if lo == 0 || c.Low < lo {
			lo = c.Low
		}
	}
	return hi, lo
}

func VWAPDeviation(price, vwap float64) float64 {
	if vwap <= 0 {
		return 0
	}
	return (price - vwap) / vwap * 100
}
