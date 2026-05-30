package risk

import (
	"context"
	"sync"
	"time"

	"encore.app/config"
)

type Snapshot struct {
	TradesToday          int
	ConsecutiveLosses    int
	DailyPnL             float64
	WeeklyPnL            float64
	CooldownUntil        time.Time
	KillSwitch           bool
	Paused               bool
	OpenPositions        int
	TradingEnabled       bool
	MarketDataHealthy    bool
	StateMismatch        bool
}

type Engine struct {
	mu sync.RWMutex
	s  Snapshot
}

func NewEngine() *Engine {
	return &Engine{s: Snapshot{MarketDataHealthy: true, TradingEnabled: false}}
}

func (e *Engine) Get() Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.s
}

func (e *Engine) SetTradingEnabled(v bool) {
	e.mu.Lock()
	e.s.TradingEnabled = v
	e.mu.Unlock()
}

func (e *Engine) SetMarketHealthy(v bool) {
	e.mu.Lock()
	e.s.MarketDataHealthy = v
	e.mu.Unlock()
}

func (e *Engine) SetOpenPositions(n int) {
	e.mu.Lock()
	e.s.OpenPositions = n
	e.mu.Unlock()
}

func (e *Engine) Pause() {
	e.mu.Lock()
	e.s.Paused = true
	e.mu.Unlock()
}

func (e *Engine) Resume() {
	e.mu.Lock()
	e.s.Paused = false
	e.mu.Unlock()
}

func (e *Engine) Kill() {
	e.mu.Lock()
	e.s.KillSwitch = true
	e.s.Paused = true
	e.mu.Unlock()
}

func (e *Engine) RecordTradeResult(netPnL float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.s.TradesToday++
	e.s.DailyPnL += netPnL
	e.s.WeeklyPnL += netPnL
	if netPnL < 0 {
		e.s.ConsecutiveLosses++
		e.s.CooldownUntil = time.Now().Add(config.CooldownAfterLossMins * time.Minute)
	} else {
		e.s.ConsecutiveLosses = 0
	}
}

func (e *Engine) ResetDailyIfNeeded(now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	// simplistic UTC midnight reset
	if now.Hour() == 0 && now.Minute() < 2 {
		e.s.TradesToday = 0
		e.s.DailyPnL = 0
	}
}

func (e *Engine) AllowNewEntry(ctx context.Context) (bool, string) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.s
	if s.KillSwitch {
		return false, "kill_switch"
	}
	if s.Paused {
		return false, "paused"
	}
	if !s.TradingEnabled {
		return false, "trading_disabled"
	}
	if !s.MarketDataHealthy {
		return false, "market_data_unhealthy"
	}
	if s.StateMismatch {
		return false, "state_mismatch"
	}
	cfg := config.Live.Get()
	if s.OpenPositions >= cfg.MaxOpenPositions {
		return false, "max_positions"
	}
	if s.TradesToday >= cfg.MaxTradesPerDay {
		return false, "max_trades_per_day"
	}
	if s.DailyPnL <= -cfg.DailyHardStopUSD {
		return false, "daily_hard_stop"
	}
	if s.WeeklyPnL <= -cfg.WeeklyHardStopUSD {
		return false, "weekly_hard_stop"
	}
	if s.ConsecutiveLosses >= config.MaxConsecutiveLosses {
		return false, "consecutive_losses"
	}
	if time.Now().Before(s.CooldownUntil) {
		return false, "cooldown"
	}
	return true, "ok"
}
