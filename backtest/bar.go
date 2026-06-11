package backtest

import (
	"time"

	"encore.app/market"
)

// Bar is one closed 5m futures candle with taker-flow split.
type Bar struct {
	OpenTime    time.Time
	CloseTime   time.Time
	Open        float64
	High        float64
	Low         float64
	Close       float64
	Volume      float64
	QuoteVol    float64
	TakerBuyVol float64 // quote asset
}

func (b Bar) TakerSellVol() float64 {
	if b.QuoteVol <= b.TakerBuyVol {
		return 0
	}
	return b.QuoteVol - b.TakerBuyVol
}

func (b Bar) Candle() market.Candle {
	return market.Candle{
		Open: b.Open, High: b.High, Low: b.Low, Close: b.Close,
		Volume: b.Volume, CloseTime: b.CloseTime,
	}
}
