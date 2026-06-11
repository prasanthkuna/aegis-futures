package signal

import (
	"encore.app/config"
	"encore.app/model"
	"encore.app/risk"
)

type GateInput struct {
	RiskSnap      risk.Snapshot
	InFundingWin  bool
	SpreadBps     float64
	MaxSpreadBps  float64
	LiqDistancePct float64
}

func EntryBlocked(in GateInput) (bool, string) {
	if in.RiskSnap.KillSwitch {
		return true, "kill_switch"
	}
	if in.RiskSnap.Paused {
		return true, "paused"
	}
	if !in.RiskSnap.TradingEnabled {
		return true, "trading_disabled"
	}
	if !in.RiskSnap.MarketDataHealthy {
		return true, "market_data_unhealthy"
	}
	cfg := config.Live.Get()
	if in.RiskSnap.OpenPositions >= cfg.MaxOpenPositions {
		return true, "max_positions"
	}
	if in.RiskSnap.TradesToday >= cfg.MaxTradesPerDay {
		return true, "max_trades_per_day"
	}
	if in.RiskSnap.DailyPnL <= -cfg.DailyHardStopUSD {
		return true, "daily_hard_stop"
	}
	if in.RiskSnap.WeeklyPnL <= -cfg.WeeklyHardStopUSD {
		return true, "weekly_hard_stop"
	}
	if in.InFundingWin {
		return true, "funding_window"
	}
	if in.SpreadBps > in.MaxSpreadBps {
		return true, "spread"
	}
	if in.LiqDistancePct > 0 && in.LiqDistancePct < 25 {
		return true, "liq_distance"
	}
	return false, ""
}

func LiqDistancePct(side model.Side, entry, mark, lev int) float64 {
	_ = side
	if entry <= 0 || mark <= 0 || lev <= 0 {
		return 100
	}
	// rough maintenance margin distance proxy
	movePct := 100.0 / float64(lev)
	return movePct
}
