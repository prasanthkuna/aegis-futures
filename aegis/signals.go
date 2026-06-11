package aegis

import (
	"context"
	"time"

	"encore.app/config"
	"encore.app/model"
	"encore.app/signal"
)

func (s *Service) rankSignals(ctx context.Context) signal.RankOutput {
	btc := s.rt.Hub.BTC5mChangePct()
	riskSnap := s.rt.Risk.Get()
	live := config.Live.Get()
	armed := tradingEnabled() && !riskSnap.Paused && !riskSnap.KillSwitch
	openN := riskSnap.OpenPositions
	if openN == 0 && s.rt.OpenPosition() != nil {
		openN = 1
	}
	canTrade := armed && openN < live.MaxOpenPositions && riskSnap.TradesToday < live.MaxTradesPerDay
	inPos := s.rt.OpenPosition() != nil

	minTrades := live.MinTradesPerDay
	if minTrades <= 0 {
		minTrades = config.MinTradesPerDay
	}

	var inputs []signal.SymbolInput
	for _, sym := range s.rt.Universe.ActiveSymbols() {
		st, ok := s.rt.Hub.Snapshot(sym)
		if !ok {
			continue
		}
		inputs = append(inputs, signal.SymbolInput{
			Symbol: sym, State: st, CoinGlassScore: s.rt.CoinGlassScore(sym),
			BTCChange5mPct: btc, IsCore: isCoreSymbol(sym),
		})
	}
	return signal.Rank(signal.RankInput{
		Now: time.Now().UTC(), Symbols: inputs,
		TradesToday: riskSnap.TradesToday, MinTradesPerDay: minTrades,
		Armed: armed, CanTrade: canTrade, InPosition: inPos,
	})
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
	tier := "signal"
	if sig.Strength >= 85 {
		tier = "A+"
	} else if sig.Strength >= floor {
		tier = "A"
	} else if sig.Strength >= floor-8 {
		tier = "B"
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
		GapToTrade: gap, WeakestLink: weakestFromComponents(sig.Components),
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
	Signals []model.ProSignal      `json:"signals"`
	Session model.SessionCockpit   `json:"session"`
	Floor   int                    `json:"floor"`
	Regime  model.RadarRegime      `json:"regime"`
}

//encore:api public method=GET path=/signals
func (s *Service) Signals(ctx context.Context) (*SignalsResponse, error) {
	out := s.rankSignals(ctx)
	if out.Signals == nil {
		out.Signals = []model.ProSignal{}
	}
	sess := out.Session
	riskSnap := s.rt.Risk.Get()
	sess.Armed = tradingEnabled() && !riskSnap.Paused && !riskSnap.KillSwitch
	sess.TradingEnabled = tradingEnabled()
	return &SignalsResponse{
		Signals: out.Signals, Session: sess, Floor: out.Floor, Regime: out.Regime,
	}, nil
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
	if !pos.HasStop {
		guard = "stop_missing"
	}
	rules := []string{"hard_stop"}
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
	return &PositionLiveResponse{
		HasPosition: true,
		Position: model.PositionLive{
			Symbol: pos.Symbol, Side: pos.Side, EntryPrice: pos.EntryPrice, MarkPrice: mark,
			Quantity: pos.Quantity, RemainingQty: qty, Leverage: live.MaxLeverage,
			StopPrice: pos.StopPrice, TakeProfitPrice: pos.TakeProfitPrice,
			UnrealizedPnL: upnl, RMultiple: rMult, HoldSec: hold,
			Playbook: pos.Playbook, StrengthAtEntry: pos.StrengthAtEntry,
			ExitPhase: pos.ExitPhase, PeakR: pos.PeakR, RulesArmed: rules,
			PartialPct: pos.PartialPct, GuardianStatus: guard,
		},
	}, nil
}
