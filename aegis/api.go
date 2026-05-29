package aegis

import (
	"context"
	"encoding/json"
	"time"

	"encore.app/binanceex"
	"encore.app/config"
	"encore.app/model"
	"encore.app/strategy"
)

type StatusResponse struct {
	Status         string `json:"status"`
	State          string `json:"state"`
	TradingEnabled bool   `json:"tradingEnabled"`
	Testnet        bool   `json:"testnet"`
	Env            string `json:"env"`
}

//encore:api public method=GET path=/status
func (s *Service) Status(ctx context.Context) (*StatusResponse, error) {
	return &StatusResponse{
		Status: "ok", State: string(s.rt.State()),
		TradingEnabled: tradingEnabled(), Testnet: useTestnet(),
		Env: secrets.AegisEnv,
	}, nil
}

//encore:api public method=GET path=/dashboard/summary
func (s *Service) DashboardSummary(ctx context.Context) (*model.DashboardSummary, error) {
	today, week, fees, _ := s.rt.Ledger.RealizedPnLSummary(ctx)
	sum := &model.DashboardSummary{
		Mode:             "live",
		BotStatus:        string(s.rt.State()),
		ActiveCapitalUsd: config.ActiveCapitalUSD,
		RealizedPnL:      week,
		NetPnLAfterFees:  week - fees,
		TodayPnL:         today,
		WeeklyPnL:        week,
		FeesPaid:         fees,
		KillSwitchActive: s.rt.Risk.Get().KillSwitch,
		TradingEnabled:   tradingEnabled(),
		Testnet:          useTestnet(),
	}
	if s.rt.Binance != nil {
		if acct, err := s.rt.Binance.Account(ctx); err == nil {
			sum.AccountBalance = binanceex.ParseFloat(acct.TotalWalletBalance)
			sum.AvailableMargin = binanceex.ParseFloat(acct.AvailableBalance)
		}
	}
	if pos := s.rt.OpenPosition(); pos != nil {
		sum.HasOpenPosition = true
		sum.LastTradeSymbol = pos.Symbol
	}
	return sum, nil
}

type RadarResponse struct {
	Items []model.SymbolSnapshot `json:"items"`
}

//encore:api public method=GET path=/radar
func (s *Service) Radar(ctx context.Context) (*RadarResponse, error) {
	var items []model.SymbolSnapshot
	btc := s.rt.Hub.BTC5mChangePct()
	for _, sym := range s.rt.Universe.ActiveSymbols() {
		st, ok := s.rt.Hub.Snapshot(sym)
		if !ok {
			continue
		}
		cg := s.rt.CoinGlassScore(sym)
		res := strategy.Evaluate(strategy.Input{
			Symbol: sym, State: st, CoinGlassScore: cg, BTCChange5mPct: btc,
		})
		cvdState := "flat"
		if res.CVDComponent >= 0.55 {
			cvdState = "up"
		} else if res.CVDComponent <= 0.35 {
			cvdState = "down"
		}
		items = append(items, model.SymbolSnapshot{
			Symbol: sym, Price: st.LastPrice, QuoteVolume24h: st.QuoteVolume24h,
			SpreadBps: st.SpreadBps, VolumeSurge: res.VolumeComponent,
			CVDState: cvdState, CoinGlassScore: cg,
			SessionScore: res.SessionComponent, TradeScore: res.TradeScore,
			Decision: res.Decision, Reason: res.Reason, UpdatedAt: time.Now().UTC(),
		})
	}
	return &RadarResponse{Items: items}, nil
}

type OpenPositionsResponse struct {
	Positions []model.OpenPositionView `json:"positions"`
}

//encore:api public method=GET path=/positions/open
func (s *Service) OpenPositions(ctx context.Context) (*OpenPositionsResponse, error) {
	pos := s.rt.OpenPosition()
	if pos == nil {
		return &OpenPositionsResponse{}, nil
	}
	st, _ := s.rt.Hub.Snapshot(pos.Symbol)
	return &OpenPositionsResponse{Positions: []model.OpenPositionView{{
		Symbol: pos.Symbol, Side: pos.Side, CurrentPrice: st.LastPrice,
		Quantity: pos.Quantity, Leverage: config.MaxLeverage, StopPrice: pos.StopPrice,
		GuardianStatus: "active",
	}}}, nil
}

type ClosedTrade struct {
	ID         int64      `json:"id"`
	Symbol     string     `json:"symbol"`
	Side       string     `json:"side"`
	EntryTime  time.Time  `json:"entryTime"`
	ExitTime   *time.Time `json:"exitTime"`
	EntryPrice float64    `json:"entryPrice"`
	ExitPrice  *float64   `json:"exitPrice"`
	Quantity   float64    `json:"quantity"`
	NetPnL     float64    `json:"netPnl"`
	Fees       float64    `json:"fees"`
	ExitReason string     `json:"exitReason"`
	TradeScore float64    `json:"tradeScore"`
}

type TradesResponse struct {
	Trades []ClosedTrade `json:"trades"`
}

//encore:api public method=GET path=/trades/closed
func (s *Service) ClosedTrades(ctx context.Context) (*TradesResponse, error) {
	rows, err := db.Query(ctx, `
		SELECT id, symbol, side, entry_time, exit_time, entry_price, exit_price,
			quantity, net_pnl, fees, exit_reason, trade_score
		FROM trades WHERE exit_time IS NOT NULL
		ORDER BY exit_time DESC LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var trades []ClosedTrade
	for rows.Next() {
		var t ClosedTrade
		if err := rows.Scan(&t.ID, &t.Symbol, &t.Side, &t.EntryTime, &t.ExitTime,
			&t.EntryPrice, &t.ExitPrice, &t.Quantity, &t.NetPnL, &t.Fees,
			&t.ExitReason, &t.TradeScore); err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return &TradesResponse{Trades: trades}, nil
}

type PnLPoint struct {
	Period string  `json:"period"`
	NetPnL float64 `json:"netPnl"`
}

type PnLPointResponse struct {
	Points []PnLPoint `json:"points"`
}

//encore:api public method=GET path=/pnl/daily
func (s *Service) PnLDaily(ctx context.Context) (*PnLPointResponse, error) {
	return s.pnlSeries(ctx, "day")
}

//encore:api public method=GET path=/pnl/weekly
func (s *Service) PnLWeekly(ctx context.Context) (*PnLPointResponse, error) {
	return s.pnlSeries(ctx, "week")
}

func (s *Service) pnlSeries(ctx context.Context, grain string) (*PnLPointResponse, error) {
	rows, err := db.Query(ctx, `
		SELECT date_trunc($1, exit_time AT TIME ZONE 'UTC')::text, COALESCE(SUM(net_pnl),0)
		FROM trades WHERE exit_time IS NOT NULL
		GROUP BY 1 ORDER BY 1 DESC LIMIT 30
	`, grain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var points []PnLPoint
	for rows.Next() {
		var p PnLPoint
		if err := rows.Scan(&p.Period, &p.NetPnL); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return &PnLPointResponse{Points: points}, nil
}

type RiskEvent struct {
	ID          int64     `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Severity    string    `json:"severity"`
	Type        string    `json:"type"`
	Symbol      *string   `json:"symbol"`
	Message     string    `json:"message"`
	ActionTaken *string   `json:"actionTaken"`
	Resolved    bool      `json:"resolved"`
}

type RiskEventsResponse struct {
	Events []RiskEvent `json:"events"`
}

//encore:api public method=GET path=/risk-events
func (s *Service) RiskEvents(ctx context.Context) (*RiskEventsResponse, error) {
	rows, err := db.Query(ctx, `
		SELECT id, timestamp, severity, type, symbol, message, action_taken, resolved
		FROM risk_events ORDER BY created_at DESC LIMIT 50
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []RiskEvent
	for rows.Next() {
		var e RiskEvent
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Severity, &e.Type, &e.Symbol,
			&e.Message, &e.ActionTaken, &e.Resolved); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return &RiskEventsResponse{Events: events}, nil
}

type ConfigResponse struct {
	AccountCapitalUsd float64 `json:"accountCapitalUsd"`
	ActiveCapitalUsd  float64 `json:"activeCapitalUsd"`
	MaxLeverage       int     `json:"maxLeverage"`
	RiskPerTradeUsd   float64 `json:"riskPerTradeUsd"`
	MaxOpenPositions  int     `json:"maxOpenPositions"`
	MaxTradesPerDay   int     `json:"maxTradesPerDay"`
	DailyHardStopUsd  float64 `json:"dailyHardStopUsd"`
	WeeklyHardStopUsd float64 `json:"weeklyHardStopUsd"`
	MinTradeScore     float64 `json:"minTradeScore"`
}

//encore:api public method=GET path=/config/current
func (s *Service) CurrentConfig(ctx context.Context) (*ConfigResponse, error) {
	var c ConfigResponse
	err := db.QueryRow(ctx, `
		SELECT account_capital_usd, active_capital_usd, max_leverage, risk_per_trade_usd,
			max_open_positions, max_trades_per_day, daily_hard_stop_usd, weekly_hard_stop_usd,
			min_trade_score
		FROM bot_config WHERE id = 'default'
	`).Scan(&c.AccountCapitalUsd, &c.ActiveCapitalUsd, &c.MaxLeverage, &c.RiskPerTradeUsd,
		&c.MaxOpenPositions, &c.MaxTradesPerDay, &c.DailyHardStopUsd, &c.WeeklyHardStopUsd,
		&c.MinTradeScore)
	return &c, err
}

type UpdateConfigRequest struct {
	ActiveCapitalUsd *float64 `json:"activeCapitalUsd,omitempty"`
	RiskPerTradeUsd  *float64 `json:"riskPerTradeUsd,omitempty"`
}

//encore:api public method=POST path=/config/update
func (s *Service) UpdateConfig(ctx context.Context, req *UpdateConfigRequest) (*ConfigResponse, error) {
	if req.ActiveCapitalUsd != nil {
		_, _ = db.Exec(ctx, `UPDATE bot_config SET active_capital_usd = $1, updated_at = NOW() WHERE id = 'default'`, *req.ActiveCapitalUsd)
	}
	if req.RiskPerTradeUsd != nil {
		_, _ = db.Exec(ctx, `UPDATE bot_config SET risk_per_trade_usd = $1, updated_at = NOW() WHERE id = 'default'`, *req.RiskPerTradeUsd)
	}
	payload, _ := json.Marshal(req)
	_, _ = db.Exec(ctx, `INSERT INTO config_versions (payload) VALUES ($1)`, payload)
	return s.CurrentConfig(ctx)
}

//encore:api public method=POST path=/bot/start
func (s *Service) BotStart(ctx context.Context) (*StatusResponse, error) {
	s.rt.Risk.Resume()
	s.rt.Risk.SetTradingEnabled(tradingEnabled())
	s.rt.SetState(ctx, model.StateScanning, "manual_start")
	return s.Status(ctx)
}

//encore:api public method=POST path=/bot/pause
func (s *Service) BotPause(ctx context.Context) (*StatusResponse, error) {
	s.rt.Risk.Pause()
	s.rt.SetState(ctx, model.StatePaused, "manual_pause")
	_ = s.rt.Telegram.Send(ctx, "BOT_PAUSED", "operator pause")
	return s.Status(ctx)
}

//encore:api public method=POST path=/bot/kill
func (s *Service) BotKill(ctx context.Context) (*StatusResponse, error) {
	s.rt.Risk.Kill()
	s.rt.SetState(ctx, model.StateKillSwitch, "manual_kill")
	_ = s.rt.Telegram.Send(ctx, "KILL_SWITCH", "operator kill")
	return s.Status(ctx)
}
