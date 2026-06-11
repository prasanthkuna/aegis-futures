package market

import (
	"sync"
	"time"

	"encore.app/binanceex"
	"encore.app/config"
)

type Candle struct {
	Open, High, Low, Close, Volume float64
	CloseTime                      time.Time
}

type SymbolState struct {
	Symbol         string
	LastPrice      float64
	Bid            float64
	Ask            float64
	QuoteVolume24h float64
	SpreadBps      float64
	Candles5m      []Candle
	TakerBuyVol    float64
	TakerSellVol   float64
	FlowUpdated    time.Time
	// Binance context (OI/funding/positioning) — zero when unavailable.
	OpenInterest        float64
	OIDeltaPct        float64 // vs ~15m ago
	FundingRate         float64
	TakerBuySellRatio   float64
	LongShortRatio      float64
	mu             sync.RWMutex
}

type Hub struct {
	mu      sync.RWMutex
	symbols map[string]*SymbolState
	btc5m   []float64
}

func NewHub() *Hub {
	return &Hub{symbols: make(map[string]*SymbolState)}
}

func (h *Hub) Ensure(symbol string) *SymbolState {
	h.mu.Lock()
	defer h.mu.Unlock()
	if st, ok := h.symbols[symbol]; ok {
		return st
	}
	st := &SymbolState{Symbol: symbol, Candles5m: make([]Candle, 0, config.SwingLookback+2)}
	h.symbols[symbol] = st
	return st
}

func (h *Hub) OnAggTrade(t binanceex.AggTrade) {
	st := h.Ensure(t.Symbol)
	st.mu.Lock()
	defer st.mu.Unlock()
	st.LastPrice = t.Price
	if t.IsBuyer {
		st.TakerBuyVol += t.Quantity * t.Price
	} else {
		st.TakerSellVol += t.Quantity * t.Price
	}
	st.FlowUpdated = t.EventTime
	if t.Symbol == "BTCUSDT" {
		h.appendBTC(t.Price)
	}
}

func (h *Hub) appendBTC(price float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.btc5m = append(h.btc5m, price)
	if len(h.btc5m) > 120 {
		h.btc5m = h.btc5m[len(h.btc5m)-120:]
	}
}

func (h *Hub) OnBookTicker(b binanceex.BookTicker) {
	st := h.Ensure(b.Symbol)
	st.mu.Lock()
	defer st.mu.Unlock()
	st.Bid, st.Ask = b.BidPrice, b.AskPrice
	if b.BidPrice > 0 && b.AskPrice > 0 {
		mid := (b.BidPrice + b.AskPrice) / 2
		st.SpreadBps = (b.AskPrice - b.BidPrice) / mid * 10000
		st.LastPrice = mid
	}
}

func (h *Hub) OnKline(k binanceex.Kline) {
	if !k.Closed {
		return
	}
	st := h.Ensure(k.Symbol)
	st.mu.Lock()
	defer st.mu.Unlock()
	st.Candles5m = append(st.Candles5m, Candle{
		Open: k.Open, High: k.High, Low: k.Low, Close: k.Close,
		Volume: k.Volume, CloseTime: k.CloseTime,
	})
	max := config.SwingLookback + 2
	if config.IsCoreSwingMode() || isCoreSymbol(k.Symbol) {
		max = config.CoreSwing5mKeep
	}
	if len(st.Candles5m) > max {
		st.Candles5m = st.Candles5m[len(st.Candles5m)-max:]
	}
}

func isCoreSymbol(symbol string) bool {
	for _, s := range config.AlwaysInclude {
		if s == symbol {
			return true
		}
	}
	return false
}

// SeedCandles5m replaces 5m history (REST bootstrap).
func (h *Hub) SeedCandles5m(symbol string, bars []binanceex.KlineBar) {
	if len(bars) == 0 {
		return
	}
	st := h.Ensure(symbol)
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]Candle, len(bars))
	for i, b := range bars {
		out[i] = Candle{
			Open: b.Open, High: b.High, Low: b.Low, Close: b.Close,
			Volume: b.Volume, CloseTime: b.CloseTime,
		}
	}
	max := config.CoreSwing5mKeep
	if len(out) > max {
		out = out[len(out)-max:]
	}
	st.Candles5m = out
}

// Candles1h returns resampled 1h bars and the latest 1h close time.
func (h *Hub) Candles1h(symbol string) ([]Candle, time.Time, bool) {
	st, ok := h.Snapshot(symbol)
	if !ok || len(st.Candles5m) < config.CoreSwing1hBars*55 {
		return nil, time.Time{}, false
	}
	h1 := Resample5mTo1h(st.Candles5m, config.CoreSwing1hBars)
	if len(h1) < 55 {
		return nil, time.Time{}, false
	}
	last := h1[len(h1)-1].CloseTime
	return h1, last, true
}

func (h *Hub) SetQuoteVolume(symbol string, vol float64) {
	st := h.Ensure(symbol)
	st.mu.Lock()
	st.QuoteVolume24h = vol
	st.mu.Unlock()
}

// SetMarketSnapshot seeds price/volume from REST (e.g. on universe refresh).
func (h *Hub) SetMarketSnapshot(symbol string, price, quoteVol float64) {
	st := h.Ensure(symbol)
	st.mu.Lock()
	if quoteVol > 0 {
		st.QuoteVolume24h = quoteVol
	}
	if price > 0 {
		st.LastPrice = price
		if st.Bid <= 0 || st.Ask <= 0 {
			st.Bid, st.Ask = price, price
		}
	}
	st.mu.Unlock()
}

func (h *Hub) Snapshot(symbol string) (SymbolState, bool) {
	h.mu.RLock()
	st, ok := h.symbols[symbol]
	h.mu.RUnlock()
	if !ok {
		return SymbolState{}, false
	}
	st.mu.RLock()
	defer st.mu.RUnlock()
	cp := *st
	cp.Candles5m = append([]Candle(nil), st.Candles5m...)
	return cp, true
}

func (h *Hub) BTC5mChangePct() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.btc5m) < 2 {
		return 0
	}
	start := h.btc5m[0]
	end := h.btc5m[len(h.btc5m)-1]
	if start == 0 {
		return 0
	}
	return (end - start) / start * 100
}

func (h *Hub) ResetFlowOlderThan(cutoff time.Time) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, st := range h.symbols {
		st.mu.Lock()
		if st.FlowUpdated.Before(cutoff) {
			st.TakerBuyVol, st.TakerSellVol = 0, 0
		}
		st.mu.Unlock()
	}
}

func (h *Hub) ListTracked() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]string, 0, len(h.symbols))
	for s := range h.symbols {
		out = append(out, s)
	}
	return out
}
