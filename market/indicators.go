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

// RSI returns Wilder RSI for the last candle (period typically 14).
func RSI(candles []Candle, period int) float64 {
	if len(candles) < period+1 || period <= 0 {
		return 50
	}
	var gain, loss float64
	for i := len(candles) - period; i < len(candles); i++ {
		d := candles[i].Close - candles[i-1].Close
		if d > 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	if loss <= 0 {
		return 100
	}
	rs := (gain / float64(period)) / (loss / float64(period))
	return 100 - (100 / (1 + rs))
}

// BollingerBands on typical price; returns lower, mid, upper for the last bar.
func BollingerBands(candles []Candle, period int, stds float64) (lower, mid, upper float64) {
	if len(candles) < period || period <= 0 {
		return 0, 0, 0
	}
	start := len(candles) - period
	var sum float64
	for i := start; i < len(candles); i++ {
		sum += (candles[i].High + candles[i].Low + candles[i].Close) / 3
	}
	mid = sum / float64(period)
	var varSum float64
	for i := start; i < len(candles); i++ {
		tp := (candles[i].High + candles[i].Low + candles[i].Close) / 3
		d := tp - mid
		varSum += d * d
	}
	sd := math.Sqrt(varSum / float64(period))
	lower = mid - stds*sd
	upper = mid + stds*sd
	return lower, mid, upper
}
