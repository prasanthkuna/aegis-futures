package aegis

import (
	"context"

	"encore.app/config"
)

func loadBotConfig(ctx context.Context) error {
	var s config.Snapshot
	err := db.QueryRow(ctx, `
		SELECT active_capital_usd, risk_per_trade_usd, min_trade_score, max_leverage,
			max_open_positions, max_trades_per_day, daily_hard_stop_usd, weekly_hard_stop_usd
		FROM bot_config WHERE id = 'default'
	`).Scan(
		&s.ActiveCapitalUSD, &s.RiskPerTradeUSD, &s.MinTradeScore, &s.MaxLeverage,
		&s.MaxOpenPositions, &s.MaxTradesPerDay, &s.DailyHardStopUSD, &s.WeeklyHardStopUSD,
	)
	if err != nil {
		config.Live.ApplyDefaults()
		return err
	}
	config.Live.Apply(s)
	return nil
}
