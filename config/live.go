package config

import "sync"

// Snapshot is the trading/risk config used by engine, strategy, and risk.
type Snapshot struct {
	ActiveCapitalUSD  float64
	RiskPerTradeUSD   float64
	MinTradeScore     float64
	MaxLeverage       int
	MaxOpenPositions  int
	MaxTradesPerDay   int
	DailyHardStopUSD  float64
	WeeklyHardStopUSD float64
}

// Live holds DB-backed settings; falls back to package defaults until loaded.
var Live liveSettings

type liveSettings struct {
	mu sync.RWMutex
	s  Snapshot
}

func init() {
	Live.ApplyDefaults()
}

func (l *liveSettings) ApplyDefaults() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.s = Snapshot{
		ActiveCapitalUSD:  ActiveCapitalUSD,
		RiskPerTradeUSD:   RiskPerTradeUSD,
		MinTradeScore:     MinTradeScore,
		MaxLeverage:       MaxLeverage,
		MaxOpenPositions:  MaxOpenPositions,
		MaxTradesPerDay:   MaxTradesPerDay,
		DailyHardStopUSD:  DailyHardStopUSD,
		WeeklyHardStopUSD: WeeklyHardStopUSD,
	}
}

func (l *liveSettings) Apply(s Snapshot) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.s = s
}

func (l *liveSettings) Get() Snapshot {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.s
}
