package aegis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"encore.app/config"
	"encore.app/model"
	"encore.app/signal"
)

func (s *Service) rankSignals(ctx context.Context) signal.RankOutput {
	_ = ctx
	return s.rt.RankSignalsAt(time.Now().UTC())
}

func proSignalToSnapshot(sig model.ProSignal, floor int, openN, maxOpen int, inPosSym string) model.SymbolSnapshot {
	score := float64(sig.Strength) / 100
	floorF := float64(floor) / 100
	gap := floorF - score
	if gap < 0 {
		gap = 0
	}
	dec := "signal"
	if sig.WillFire {
		dec = "trade"
	} else if score >= floorF*0.9 {
		dec = "watch"
	}
	tier := sig.Tier
	if tier == "" {
		tier = "signal"
	}
	reason := sig.Playbook
	if sig.Extra.CVDState != "" {
		reason += " · " + sig.Extra.TakerFlow + "/" + sig.Extra.CVDState
	}
	hasSlot := openN < maxOpen && sig.Symbol != inPosSym
	return model.SymbolSnapshot{
		Symbol: sig.Symbol, Rank: sig.Rank, QuoteVolume24h: sig.QuoteVol24,
		Price: sig.Price, SpreadBps: sig.SpreadBps, VolumeSurge: sig.Components.Volume,
		CVDState: sig.Extra.CVDState, TakerFlow: sig.Extra.TakerFlow,
		OIFundingContext: oiFundingContext(sig.Extra.FaTilt / 100),
		CoinGlassScore: sig.Extra.FaTilt / 100, SessionScore: sig.Components.Session,
		TradeScore: score, Decision: dec, Reason: reason, UpdatedAt: sig.UpdatedAt,
		GapToTrade: gap, WeakestLink: sig.WeakestLink,
		Tier: tier, SideHint: string(sig.Side),
		Components: sig.Components,
		Gates: model.GateFlags{
			MinScore:  sig.Strength >= floor,
			Structure: sig.Components.Structure >= 0.5,
			Flow:      sig.Components.CVD >= 0.35,
			Btc:       true,
			Spread:    sig.SpreadBps <= 10,
		},
		BtcRegime: sig.Extra.BtcRegime, IsCore: sig.IsCore, WillFire: sig.WillFire,
		HasOpenSlot: hasSlot,
	}
}

func weakestFromComponents(c model.ScoreComponents) string {
	type kv struct {
		k string
		v float64
	}
	parts := []kv{
		{"volume", c.Volume}, {"cvd", c.CVD}, {"structure", c.Structure},
		{"context", c.Context}, {"depth", c.Depth}, {"session", c.Session},
	}
	w := parts[0]
	for _, p := range parts[1:] {
		if p.v < w.v {
			w = p
		}
	}
	return w.k
}

type SignalsResponse struct {
	Universe  []model.ProSignal     `json:"universe"`
	Signals   []model.ProSignal     `json:"signals"`
	NearMiss  []model.ProSignal     `json:"nearMiss"`
	Session   model.SessionCockpit  `json:"session"`
	Floor     int                   `json:"floor"`
	Regime    model.RadarRegime     `json:"regime"`
	Heartbeat model.EngineHeartbeat `json:"heartbeat"`
	Narrative string                `json:"narrative"`
}

//encore:api public method=GET path=/signals
func (s *Service) Signals(ctx context.Context) (*SignalsResponse, error) {
	out := s.rankSignals(ctx)
	resp := &SignalsResponse{
		Universe:  out.Universe,
		Signals:   out.Signals,
		NearMiss:  out.NearMiss,
		Session:   out.Session,
		Floor:     out.Floor,
		Regime:    out.Regime,
		Heartbeat: out.Heartbeat,
		Narrative: out.Narrative,
	}
	if resp.Universe == nil {
		resp.Universe = []model.ProSignal{}
	}
	if resp.Signals == nil {
		resp.Signals = []model.ProSignal{}
	}
	if resp.NearMiss == nil {
		resp.NearMiss = []model.ProSignal{}
	}
	riskSnap := s.rt.Risk.Get()
	resp.Session.Armed = tradingEnabled() && !riskSnap.Paused && !riskSnap.KillSwitch
	resp.Session.TradingEnabled = tradingEnabled()
	resp.Session.BtcChange5mPct = out.Regime.BtcChange5mPct
	return resp, nil
}

//encore:api public method=GET path=/signals/session
func (s *Service) SignalsSession(ctx context.Context) (*model.SessionCockpit, error) {
	out := s.rankSignals(ctx)
	sess := out.Session
	riskSnap := s.rt.Risk.Get()
	sess.Armed = tradingEnabled() && !riskSnap.Paused && !riskSnap.KillSwitch
	sess.TradingEnabled = tradingEnabled()
	sess.BtcChange5mPct = out.Regime.BtcChange5mPct
	return &sess, nil
}

type FeedResponse struct {
	Events []model.SignalFeedEvent `json:"events"`
}

//encore:api public method=GET path=/signals/feed
func (s *Service) SignalsFeed(ctx context.Context) (*FeedResponse, error) {
	events := s.rt.FeedEvents()
	if events == nil {
		events = []model.SignalFeedEvent{}
	}
	return &FeedResponse{Events: events}, nil
}

type PlaybookStatsResponse struct {
	Stats []model.PlaybookStat `json:"stats"`
}

//encore:api public method=GET path=/analytics/playbooks
func (s *Service) PlaybookStats(ctx context.Context) (*PlaybookStatsResponse, error) {
	rows, err := db.Query(ctx, `
		SELECT COALESCE(NULLIF(entry_reason,''), 'UNKNOWN') AS pb,
		       COUNT(*)::int,
		       COALESCE(AVG(CASE WHEN net_pnl > 0 THEN 1.0 ELSE 0.0 END), 0),
		       COALESCE(AVG(r_multiple), 0),
		       COALESCE(SUM(net_pnl), 0)
		FROM trades WHERE exit_time IS NOT NULL
		GROUP BY 1 ORDER BY COUNT(*) DESC
	`)
	if err != nil {
		return &PlaybookStatsResponse{Stats: []model.PlaybookStat{}}, nil
	}
	defer rows.Close()
	var stats []model.PlaybookStat
	for rows.Next() {
		var st model.PlaybookStat
		if err := rows.Scan(&st.Playbook, &st.Trades, &st.WinRate, &st.AvgR, &st.NetPnL); err != nil {
			continue
		}
		stats = append(stats, st)
	}
	if stats == nil {
		stats = []model.PlaybookStat{}
	}
	return &PlaybookStatsResponse{Stats: stats}, nil
}

type ExecuteRequest struct {
	Symbol string `json:"symbol"`
}

type ExecuteResponse struct {
	OK      bool   `json:"ok"`
	Symbol  string `json:"symbol"`
	Message string `json:"message"`
}

//encore:api public method=POST path=/execute
func (s *Service) Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResponse, error) {
	if req == nil || req.Symbol == "" {
		return nil, errors.New("symbol required")
	}
	if err := s.rt.ExecuteSignal(ctx, req.Symbol); err != nil {
		return &ExecuteResponse{OK: false, Symbol: req.Symbol, Message: err.Error()}, nil
	}
	return &ExecuteResponse{
		OK: true, Symbol: req.Symbol,
		Message: fmt.Sprintf("entry placed for %s", req.Symbol),
	}, nil
}

type PositionLiveResponse struct {
	HasPosition bool               `json:"hasPosition"`
	Position    model.PositionLive `json:"position"`
}

//encore:api public method=GET path=/position/live
func (s *Service) PositionLive(ctx context.Context) (*PositionLiveResponse, error) {
	pos := s.rt.OpenPosition()
	if pos == nil {
		return &PositionLiveResponse{HasPosition: false}, nil
	}
	live := config.Live.Get()
	st, _ := s.rt.Hub.Snapshot(pos.Symbol)
	mark := st.LastPrice
	qty := pos.Quantity
	if pos.RemainingQty > 0 {
		qty = pos.RemainingQty
	}
	upnl := unrealizedPnL(pos.Side, pos.EntryPrice, mark, qty)
	riskUSD := pos.RiskUSD
	if riskUSD <= 0 {
		riskUSD = live.RiskPerTradeUSD
	}
	rMult := 0.0
	if riskUSD > 0 {
		rMult = upnl / riskUSD
	}
	hold := int64(0)
	if pos.EntryTime > 0 {
		hold = (time.Now().UnixMilli() - pos.EntryTime) / 1000
	}
	guard := "active"
	if pos.Paper {
		guard = "paper"
	} else if !pos.HasStop {
		guard = "stop_missing"
	}
	var rules []string
	if config.IsCoreSwingMode() {
		rules = []string{"hard_stop", "core_rr_tp", "core_time_stop"}
	} else {
		rules = []string{"hard_stop"}
		switch pos.ExitPhase {
		case "BREAKEVEN":
			rules = append(rules, "be_0.5r")
		case "PARTIAL_1":
			rules = append(rules, "partial_1r", "trail_armed")
		case "TRAILING":
			rules = append(rules, "atr_trail")
		default:
			if rMult >= 0.5 {
				rules = append(rules, "be_pending")
			}
		}
	}
	return &PositionLiveResponse{
		HasPosition: true,
		Position: model.PositionLive{
			Symbol: pos.Symbol, Side: pos.Side, EntryPrice: pos.EntryPrice, MarkPrice: mark,
			Quantity: pos.Quantity, RemainingQty: qty, Leverage: live.MaxLeverage,
			StopPrice: pos.StopPrice, TakeProfitPrice: pos.TakeProfitPrice,
			UnrealizedPnL: upnl, RMultiple: rMult, HoldSec: hold,
			Playbook: pos.Playbook, StrengthAtEntry: pos.StrengthAtEntry,
			ExitPhase: pos.ExitPhase, PeakR: pos.PeakR, RulesArmed: rules,
			PartialPct: pos.PartialPct, GuardianStatus: guard, Paper: pos.Paper,
		},
	}, nil
}
