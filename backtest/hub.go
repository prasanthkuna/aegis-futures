package backtest

import (
	"encore.app/config"
	"encore.app/market"
)

// ReplayHub holds per-symbol replay state at a point in time.
type ReplayHub struct {
	states map[string]*market.SymbolState
	btc    []float64
}

func NewReplayHub() *ReplayHub {
	return &ReplayHub{states: make(map[string]*market.SymbolState)}
}

func (h *ReplayHub) ApplyBar(symbol string, b Bar, quoteVol24h float64) {
	st := h.ensure(symbol)
	st.LastPrice = b.Close
	st.Bid = b.Close * 0.9999
	st.Ask = b.Close * 1.0001
	mid := b.Close
	if mid > 0 {
		st.SpreadBps = (st.Ask - st.Bid) / mid * 10000
	}
	st.QuoteVolume24h = quoteVol24h
	st.TakerBuyVol = b.TakerBuyVol
	st.TakerSellVol = b.TakerSellVol()
	st.FlowUpdated = b.CloseTime
	st.Candles5m = append(st.Candles5m, b.Candle())
	max := config.SwingLookback + 2
	if len(st.Candles5m) > max {
		st.Candles5m = st.Candles5m[len(st.Candles5m)-max:]
	}
	if symbol == "BTCUSDT" {
		h.btc = append(h.btc, b.Close)
		if len(h.btc) > 120 {
			h.btc = h.btc[len(h.btc)-120:]
		}
	}
}

func (h *ReplayHub) ensure(symbol string) *market.SymbolState {
	if st, ok := h.states[symbol]; ok {
		return st
	}
	st := &market.SymbolState{
		Symbol: symbol, Candles5m: make([]market.Candle, 0, config.SwingLookback+2),
	}
	h.states[symbol] = st
	return st
}

func (h *ReplayHub) Snapshot(symbol string) (market.SymbolState, bool) {
	st, ok := h.states[symbol]
	if !ok {
		return market.SymbolState{}, false
	}
	cp := *st
	cp.Candles5m = append([]market.Candle(nil), st.Candles5m...)
	return cp, true
}

func (h *ReplayHub) BTC5mChangePct() float64 {
	if len(h.btc) < 2 {
		return 0
	}
	start := h.btc[0]
	end := h.btc[len(h.btc)-1]
	if start == 0 {
		return 0
	}
	return (end - start) / start * 100
}

func (h *ReplayHub) ApplyContext(symbol string, cp ContextPoint) {
	if cp.Time.IsZero() && cp.OpenInterest == 0 && cp.OIDeltaPct == 0 {
		return
	}
	st := h.ensure(symbol)
	st.OpenInterest = cp.OpenInterest
	st.OIDeltaPct = cp.OIDeltaPct
	st.FundingRate = cp.FundingRate
	st.TakerBuySellRatio = cp.TakerBuySellRatio
	st.LongShortRatio = cp.LongShortRatio
}

func (h *ReplayHub) Warmup(symbol string, bars []Bar) {
	for _, b := range bars {
		h.ApplyBar(symbol, b, 0)
	}
}
