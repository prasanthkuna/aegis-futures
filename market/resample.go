package market

import "encore.app/config"

// Resample5mTo1h aggregates n consecutive 5m candles into 1h bars.
func Resample5mTo1h(candles []Candle, n int) []Candle {
	if n <= 0 {
		n = config.CoreSwing1hBars
	}
	if len(candles) < n {
		return nil
	}
	var out []Candle
	for i := 0; i+n <= len(candles); i += n {
		chunk := candles[i : i+n]
		b := Candle{
			CloseTime: chunk[n-1].CloseTime,
			Open:      chunk[0].Open,
			Close:     chunk[n-1].Close,
			High:      chunk[0].High,
			Low:       chunk[0].Low,
			Volume:    chunk[0].Volume,
		}
		for _, c := range chunk[1:] {
			if c.High > b.High {
				b.High = c.High
			}
			if c.Low < b.Low {
				b.Low = c.Low
			}
			b.Volume += c.Volume
		}
		out = append(out, b)
	}
	return out
}
