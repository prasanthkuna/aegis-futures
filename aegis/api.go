package aegis

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"sort"
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
	Paused         bool   `json:"paused"`
	Armed          bool   `json:"armed"`
	UniverseSize   int    `json:"universeSize"`
	Testnet        bool   `json:"testnet"`
	Env            string `json:"env"`
}

type AccountHealthResponse struct {
	OK                 bool    `json:"ok"`
	TotalWalletBalance float64 `json:"totalWalletBalance"`
	AvailableBalance   float64 `json:"availableBalance"`
	Testnet            bool    `json:"testnet"`
	TradingEnabled     bool    `json:"tradingEnabled"`
	HasAPIKeys         bool    `json:"hasApiKeys"`
	Error              string  `json:"error,omitempty"`
}

//encore:api public method=GET path=/account/health
func (s *Service) AccountHealth(ctx context.Context) (*AccountHealthResponse, error) {
	out := &AccountHealthResponse{
		Testnet:        useTestnet(),
		TradingEnabled: tradingEnabled(),
		HasAPIKeys:     hasBinanceKeys(),
	}
	if !out.HasAPIKeys {
		out.Error = "missing keys: set BinanceAPIKey and BinanceAPISecret in Encore Cloud secrets"
		return out, nil
	}
	if s.rt.Binance == nil {
		out.Error = "binance client not initialized"
		return out, nil
	}
	acct, err := s.rt.Binance.Account(ctx)
	if err != nil {
		out.Error = err.Error()
		return out, nil
	}
	out.OK = true
	out.TotalWalletBalance = binanceex.ParseFloat(acct.TotalWalletBalance)
	out.AvailableBalance = binanceex.ParseFloat(acct.AvailableBalance)
	return out, nil
}

//encore:api public method=GET path=/status
func (s *Service) Status(ctx context.Context) (*StatusResponse, error) {
	snap := s.rt.Risk.Get()
	armed := tradingEnabled() && !snap.Paused && !snap.KillSwitch
	return &StatusResponse{
		Status: "ok", State: string(s.rt.State()),
		TradingEnabled: tradingEnabled(), Paused: snap.Paused, Armed: armed,
		UniverseSize: len(s.rt.Universe.ActiveSymbols()),
		Testnet: useTestnet(), Env: aegisEnv(),
	}, nil
}

//encore:api public method=GET path=/dashboard/summary
func (s *Service) DashboardSummary(ctx context.Context) (*model.DashboardSummary, error) {
	today, week, fees, _ := s.rt.Ledger.RealizedPnLSummary(ctx)
	live := config.Live.Get()
	sum := &model.DashboardSummary{
		Mode:             "live",
		BotStatus:        string(s.rt.State()),
		ActiveCapitalUsd: live.ActiveCapitalUSD,
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
	var fundingPaid float64
	_ = db.QueryRow(ctx, `
		SELECT COALESCE(SUM(funding), 0) FROM trades WHERE exit_time IS NOT NULL
	`).Scan(&fundingPaid)
	sum.FundingPaid = fundingPaid
	sum.CurrentDrawdown = s.maxDrawdown(ctx)

	if pos := s.rt.OpenPosition(); pos != nil {
		sum.HasOpenPosition = true
		sum.LastTradeSymbol = pos.Symbol
		st, ok := s.rt.Hub.Snapshot(pos.Symbol)
		if ok && pos.EntryPrice > 0 {
			sum.OpenPnL = unrealizedPnL(pos.Side, pos.EntryPrice, st.LastPrice, pos.Quantity)
		}
	} else {
		var lastSym sql.NullString
		if err := db.QueryRow(ctx, `
			SELECT symbol FROM trades
			ORDER BY COALESCE(exit_time, entry_time) DESC LIMIT 1
		`).Scan(&lastSym); err == nil && lastSym.Valid {
			sum.LastTradeSymbol = lastSym.String
		}
	}
	return sum, nil
}

func unrealizedPnL(side model.Side, entry, current, qty float64) float64 {
	if qty <= 0 || current <= 0 {
		return 0
	}
	if side == model.SideLong {
		return (current - entry) * qty
	}
	return (entry - current) * qty
}

func (s *Service) maxDrawdown(ctx context.Context) float64 {
	rows, err := db.Query(ctx, `
		SELECT net_pnl FROM trades
		WHERE exit_time IS NOT NULL
		ORDER BY exit_time ASC
	`)
	if err != nil {
		return 0
	}
	defer rows.Close()
	var peak, maxDD float64
	var cum float64
	for rows.Next() {
		var pnl float64
		if err := rows.Scan(&pnl); err != nil {
			return 0
		}
		cum += pnl
		if cum > peak {
			peak = cum
		}
		dd := peak - cum
		if dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}

type RadarResponse struct {
	Items  []model.SymbolSnapshot `json:"items"`
	Meta   model.RadarMeta        `json:"meta"`
	Regime model.RadarRegime      `json:"regime"`
}

//encore:api public method=GET path=/radar
func (s *Service) Radar(ctx context.Context) (*RadarResponse, error) {
	live := config.Live.Get()
	minScore := live.MinTradeScore
	if minScore <= 0 {
		minScore = config.MinTradeScore
	}
	watchMin := minScore * 0.85
	aplus := config.APlusTradeScore
	btc := s.rt.Hub.BTC5mChangePct()
	riskSnap := s.rt.Risk.Get()
	armed := tradingEnabled() && !riskSnap.Paused && !riskSnap.KillSwitch
	openN := riskSnap.OpenPositions
	if openN == 0 && s.rt.OpenPosition() != nil {
		openN = 1
	}
	canTrade := armed && openN < live.MaxOpenPositions && riskSnap.TradesToday < live.MaxTradesPerDay

	var items []model.SymbolSnapshot
	for _, sym := range s.rt.Universe.ActiveSymbols() {
		st, ok := s.rt.Hub.Snapshot(sym)
		if !ok {
			continue
		}
		cg := s.rt.CoinGlassScore(sym)
		res := strategy.Evaluate(strategy.Input{
			Symbol: sym, State: st, CoinGlassScore: cg, BTCChange5mPct: btc,
		})
		dec := radarDecision(res, minScore)
		gap := minScore - res.TradeScore
		if gap < 0 {
			gap = 0
		}
		pos := s.rt.OpenPosition()
		inPos := pos != nil && pos.Symbol == sym
		willFire := canTrade && dec == "trade" && !inPos
		items = append(items, model.SymbolSnapshot{
			Symbol: sym, Price: st.LastPrice, QuoteVolume24h: st.QuoteVolume24h,
			SpreadBps: st.SpreadBps, VolumeSurge: res.VolumeComponent,
			CVDState: res.CVDState, TakerFlow: res.TakerFlow,
			OIFundingContext: oiFundingContext(cg), CoinGlassScore: cg,
			SessionScore: res.SessionComponent, TradeScore: res.TradeScore,
			Decision: dec, Reason: res.Reason, UpdatedAt: time.Now().UTC(),
			GapToTrade: gap, WeakestLink: strategy.WeakestLink(res, res.Reason),
			Tier: strategy.TierLabel(res.TradeScore, minScore, aplus, dec),
			SideHint: strategy.SideHintString(res.SideHint),
			Components: strategy.ComponentsFrom(res),
			Gates:      strategy.GateFlagsFor(res, minScore, btc, st.SpreadBps),
			BtcRegime:  strategy.BtcRegimeTag(btc),
			IsCore:     isCoreSymbol(sym),
			WillFire:   willFire,
			HasOpenSlot: openN < live.MaxOpenPositions && !inPos,
		})
	}
	s.radar.applyDeltas(items)
	sortRadarActionability(items)
	for i := range items {
		items[i].Rank = i + 1
	}
	if items == nil {
		items = []model.SymbolSnapshot{}
	}
	regime := strategy.BuildRegime(items, btc)
	meta := model.RadarMeta{
		MinTradeScore: minScore, WatchMinScore: watchMin, APlusTradeScore: aplus,
		Armed: armed, TradingEnabled: tradingEnabled(), Paused: riskSnap.Paused,
		KillSwitch: riskSnap.KillSwitch, TradesToday: riskSnap.TradesToday,
		MaxTradesPerDay: live.MaxTradesPerDay, OpenPositions: openN,
		MaxOpenPositions: live.MaxOpenPositions, TodayPnL: riskSnap.DailyPnL,
		DailyHardStopUsd: live.DailyHardStopUSD,
	}
	return &RadarResponse{Items: items, Meta: meta, Regime: regime}, nil
}

func isCoreSymbol(sym string) bool {
	for _, c := range config.AlwaysInclude {
		if c == sym {
			return true
		}
	}
	return false
}

func sortRadarActionability(items []model.SymbolSnapshot) {
	priority := map[string]int{"trade": 0, "watch": 1, "skip": 2}
	sort.Slice(items, func(i, j int) bool {
		pi, pj := priority[items[i].Decision], priority[items[j].Decision]
		if pi != pj {
			return pi < pj
		}
		if items[i].GapToTrade != items[j].GapToTrade {
			return items[i].GapToTrade < items[j].GapToTrade
		}
		return items[i].TradeScore > items[j].TradeScore
	})
}

func radarDecision(res strategy.Result, minScore float64) string {
	if res.Decision == "trade" {
		return "trade"
	}
	if res.TradeScore >= minScore*0.85 {
		return "watch"
	}
	return "skip"
}

func oiFundingContext(cgScore float64) string {
	if cgScore > 0.3 {
		return "supportive"
	}
	if cgScore < -0.3 {
		return "crowded"
	}
	return "neutral"
}

type OpenPositionsResponse struct {
	Positions []model.OpenPositionView `json:"positions"`
}

//encore:api public method=GET path=/positions/open
func (s *Service) OpenPositions(ctx context.Context) (*OpenPositionsResponse, error) {
	pos := s.rt.OpenPosition()
	if pos == nil {
		return &OpenPositionsResponse{Positions: []model.OpenPositionView{}}, nil
	}
	live := config.Live.Get()
	st, _ := s.rt.Hub.Snapshot(pos.Symbol)
	upnl := unrealizedPnL(pos.Side, pos.EntryPrice, st.LastPrice, pos.Quantity)
	rMult := 0.0
	if live.RiskPerTradeUSD > 0 {
		rMult = upnl / live.RiskPerTradeUSD
	}
	secs := int64(0)
	if pos.EntryTime > 0 {
		secs = (time.Now().UnixMilli() - pos.EntryTime) / 1000
	}
	guard := "active"
	if !pos.HasStop {
		guard = "stop_missing"
	}
	return &OpenPositionsResponse{Positions: []model.OpenPositionView{{
		Symbol: pos.Symbol, Side: pos.Side, EntryPrice: pos.EntryPrice,
		CurrentPrice: st.LastPrice, Quantity: pos.Quantity, Leverage: live.MaxLeverage,
		StopPrice: pos.StopPrice, TakeProfitPrice: pos.TakeProfitPrice,
		UnrealizedPnL: upnl, RMultiple: rMult, TimeInTradeSec: secs, GuardianStatus: guard,
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
	GrossPnL   float64    `json:"grossPnl"`
	Fees       float64    `json:"fees"`
	Funding    float64    `json:"funding"`
	NetPnL     float64    `json:"netPnl"`
	RMultiple  float64    `json:"rMultiple"`
	ExitReason string     `json:"exitReason"`
	TradeScore float64    `json:"tradeScore"`
	Session    *string    `json:"session"`
	MistakeTag *string    `json:"mistakeTag"`
}

type TradesResponse struct {
	Trades []ClosedTrade `json:"trades"`
}

//encore:api public method=GET path=/trades/closed
func (s *Service) ClosedTrades(ctx context.Context) (*TradesResponse, error) {
	rows, err := db.Query(ctx, `
		SELECT id, symbol, side, entry_time, exit_time, entry_price, exit_price,
			quantity, gross_pnl, fees, funding, net_pnl, r_multiple,
			exit_reason, trade_score, session
		FROM trades WHERE exit_time IS NOT NULL
		ORDER BY exit_time DESC LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	trades := make([]ClosedTrade, 0)
	for rows.Next() {
		var t ClosedTrade
		if err := rows.Scan(&t.ID, &t.Symbol, &t.Side, &t.EntryTime, &t.ExitTime,
			&t.EntryPrice, &t.ExitPrice, &t.Quantity, &t.GrossPnL, &t.Fees, &t.Funding,
			&t.NetPnL, &t.RMultiple, &t.ExitReason, &t.TradeScore, &t.Session); err != nil {
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
	points := make([]PnLPoint, 0)
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
	events := make([]RiskEvent, 0)
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

type StrategyTruthResponse struct {
	WinRate              float64 `json:"winRate"`
	AvgWin               float64 `json:"avgWin"`
	AvgLoss              float64 `json:"avgLoss"`
	ProfitFactor         float64 `json:"profitFactor"`
	ExpectancyPerTrade   float64 `json:"expectancyPerTrade"`
	ExpectancyAfterFees  float64 `json:"expectancyAfterFees"`
	MaxDrawdown          float64 `json:"maxDrawdown"`
	FeesPctOfGrossProfit float64 `json:"feesPctOfGrossProfit"`
	BestSymbol           string  `json:"bestSymbol"`
	WorstSymbol          string  `json:"worstSymbol"`
	BestSession          string  `json:"bestSession"`
	WorstSession         string  `json:"worstSession"`
	LongPnL              float64 `json:"longPnl"`
	ShortPnL             float64 `json:"shortPnl"`
	PostOnlyFillRate     float64 `json:"postOnlyFillRate"`
	MissedTradeCount     int     `json:"missedTradeCount"`
	StopMissingIncidents int     `json:"stopMissingIncidents"`
	StateMismatchCount   int     `json:"stateMismatchCount"`
	ClosedTradeCount     int     `json:"closedTradeCount"`
}

//encore:api public method=GET path=/dashboard/strategy-truth
func (s *Service) GetStrategyTruth(ctx context.Context) (*StrategyTruthResponse, error) {
	out := &StrategyTruthResponse{}
	_ = db.QueryRow(ctx, `SELECT COUNT(*) FROM missed_trades`).Scan(&out.MissedTradeCount)
	_ = db.QueryRow(ctx, `
		SELECT COUNT(*) FROM risk_events
		WHERE type ILIKE '%stop%' OR message ILIKE '%stop%'
	`).Scan(&out.StopMissingIncidents)
	_ = db.QueryRow(ctx, `
		SELECT COUNT(*) FROM risk_events WHERE type = 'state_mismatch'
	`).Scan(&out.StateMismatchCount)

	var wins, total int
	var avgWin, avgLoss, sumWin, sumLossAbs, grossProfit, totalFees, expectancy float64
	_ = db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE net_pnl > 0),
			COUNT(*),
			COALESCE(AVG(net_pnl) FILTER (WHERE net_pnl > 0), 0),
			COALESCE(AVG(net_pnl) FILTER (WHERE net_pnl <= 0), 0),
			COALESCE(SUM(net_pnl) FILTER (WHERE net_pnl > 0), 0),
			COALESCE(SUM(ABS(net_pnl)) FILTER (WHERE net_pnl < 0), 0),
			COALESCE(SUM(gross_pnl) FILTER (WHERE gross_pnl > 0), 0),
			COALESCE(SUM(fees), 0),
			COALESCE(AVG(net_pnl), 0)
		FROM trades WHERE exit_time IS NOT NULL
	`).Scan(&wins, &total, &avgWin, &avgLoss, &sumWin, &sumLossAbs, &grossProfit, &totalFees, &expectancy)

	out.ClosedTradeCount = total
	if total > 0 {
		out.WinRate = float64(wins) / float64(total)
	}
	out.AvgWin = avgWin
	out.AvgLoss = avgLoss
	if sumLossAbs > 0 {
		out.ProfitFactor = sumWin / sumLossAbs
	}
	out.ExpectancyPerTrade = expectancy
	out.ExpectancyAfterFees = expectancy
	out.MaxDrawdown = s.maxDrawdown(ctx)
	if grossProfit > 0 {
		out.FeesPctOfGrossProfit = totalFees / grossProfit * 100
	}

	_ = db.QueryRow(ctx, `
		SELECT symbol FROM trades WHERE exit_time IS NOT NULL
		GROUP BY symbol ORDER BY SUM(net_pnl) DESC LIMIT 1
	`).Scan(&out.BestSymbol)
	_ = db.QueryRow(ctx, `
		SELECT symbol FROM trades WHERE exit_time IS NOT NULL
		GROUP BY symbol ORDER BY SUM(net_pnl) ASC LIMIT 1
	`).Scan(&out.WorstSymbol)
	_ = db.QueryRow(ctx, `
		SELECT session FROM trades WHERE exit_time IS NOT NULL AND session IS NOT NULL
		GROUP BY session ORDER BY SUM(net_pnl) DESC LIMIT 1
	`).Scan(&out.BestSession)
	_ = db.QueryRow(ctx, `
		SELECT session FROM trades WHERE exit_time IS NOT NULL AND session IS NOT NULL
		GROUP BY session ORDER BY SUM(net_pnl) ASC LIMIT 1
	`).Scan(&out.WorstSession)
	_ = db.QueryRow(ctx, `
		SELECT COALESCE(SUM(net_pnl), 0) FROM trades
		WHERE exit_time IS NOT NULL AND side = 'LONG'
	`).Scan(&out.LongPnL)
	_ = db.QueryRow(ctx, `
		SELECT COALESCE(SUM(net_pnl), 0) FROM trades
		WHERE exit_time IS NOT NULL AND side = 'SHORT'
	`).Scan(&out.ShortPnL)

	var postOnly, allOrders int
	_ = db.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE order_type ILIKE '%limit%'`).Scan(&allOrders)
	_ = db.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE status = 'FILLED' AND order_type ILIKE '%limit%'`).Scan(&postOnly)
	if allOrders > 0 {
		out.PostOnlyFillRate = float64(postOnly) / float64(allOrders)
	}
	if math.IsNaN(out.ProfitFactor) {
		out.ProfitFactor = 0
	}
	return out, nil
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
	ActiveCapitalUsd  *float64 `json:"activeCapitalUsd,omitempty"`
	RiskPerTradeUsd   *float64 `json:"riskPerTradeUsd,omitempty"`
	MinTradeScore     *float64 `json:"minTradeScore,omitempty"`
	MaxTradesPerDay   *int     `json:"maxTradesPerDay,omitempty"`
	DailyHardStopUsd  *float64 `json:"dailyHardStopUsd,omitempty"`
	WeeklyHardStopUsd *float64 `json:"weeklyHardStopUsd,omitempty"`
	MaxLeverage       *int     `json:"maxLeverage,omitempty"`
}

//encore:api public method=POST path=/config/update
func (s *Service) UpdateConfig(ctx context.Context, req *UpdateConfigRequest) (*ConfigResponse, error) {
	if req.ActiveCapitalUsd != nil {
		_, _ = db.Exec(ctx, `UPDATE bot_config SET active_capital_usd = $1, updated_at = NOW() WHERE id = 'default'`, *req.ActiveCapitalUsd)
	}
	if req.RiskPerTradeUsd != nil {
		_, _ = db.Exec(ctx, `UPDATE bot_config SET risk_per_trade_usd = $1, updated_at = NOW() WHERE id = 'default'`, *req.RiskPerTradeUsd)
	}
	if req.MinTradeScore != nil {
		v := *req.MinTradeScore
		if v > 1 {
			v = v / 100 // allow UI to send 78 meaning 0.78
		}
		_, _ = db.Exec(ctx, `UPDATE bot_config SET min_trade_score = $1, updated_at = NOW() WHERE id = 'default'`, v)
	}
	if req.MaxTradesPerDay != nil {
		_, _ = db.Exec(ctx, `UPDATE bot_config SET max_trades_per_day = $1, updated_at = NOW() WHERE id = 'default'`, *req.MaxTradesPerDay)
	}
	if req.DailyHardStopUsd != nil {
		_, _ = db.Exec(ctx, `UPDATE bot_config SET daily_hard_stop_usd = $1, updated_at = NOW() WHERE id = 'default'`, *req.DailyHardStopUsd)
	}
	if req.WeeklyHardStopUsd != nil {
		_, _ = db.Exec(ctx, `UPDATE bot_config SET weekly_hard_stop_usd = $1, updated_at = NOW() WHERE id = 'default'`, *req.WeeklyHardStopUsd)
	}
	if req.MaxLeverage != nil {
		_, _ = db.Exec(ctx, `UPDATE bot_config SET max_leverage = $1, updated_at = NOW() WHERE id = 'default'`, *req.MaxLeverage)
	}
	payload, _ := json.Marshal(req)
	_, _ = db.Exec(ctx, `INSERT INTO config_versions (payload) VALUES ($1)`, payload)
	_ = loadBotConfig(ctx)
	return s.CurrentConfig(ctx)
}

//encore:api public method=POST path=/bot/start
func (s *Service) BotStart(ctx context.Context) (*StatusResponse, error) {
	s.rt.Risk.Resume()
	s.rt.Risk.SetTradingEnabled(tradingEnabled())
	s.rt.SetState(ctx, model.StateScanning, "manual_start")
	if !tradingEnabled() {
		log.Printf("bot/start: scanning resumed; set AegisTradingEnabled=true to allow orders")
	}
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
