package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"encore.app/alerts"
	"encore.app/binanceex"
	"encore.app/coinglass"
	"encore.app/config"
	"encore.app/execution"
	"encore.app/guardian"
	"encore.app/ledger"
	"encore.app/market"
	"encore.app/model"
	"encore.app/risk"
	"encore.app/exit"
	"encore.app/signal"
	"encore.app/universe"
)

type Runtime struct {
	Hub       *market.Hub
	Universe  *universe.Manager
	Risk      *risk.Engine
	Execution *execution.Service
	Guardian  *guardian.Service
	Ledger    *ledger.Store
	Telegram  *alerts.Telegram
	CoinGlass *coinglass.Client
	Binance   *binanceex.Client

	mu            sync.RWMutex
	state         model.BotState
	runID         int64
	openPos       *guardian.InternalPosition
	lastWSHealthy time.Time
	cgScores      map[string]float64
	exitMgr       *exit.Manager
	lastRank      signal.RankOutput
	feed          []model.SignalFeedEvent
	feedMu        sync.RWMutex

	// OnUniverseChanged is called after universe refresh (e.g. resubscribe WS).
	OnUniverseChanged func(symbols []string)
}

func NewRuntime(
	hub *market.Hub,
	uni *universe.Manager,
	r *risk.Engine,
	ex *execution.Service,
	g *guardian.Service,
	led *ledger.Store,
	tg *alerts.Telegram,
	cg *coinglass.Client,
	bc *binanceex.Client,
) *Runtime {
	return &Runtime{
		Hub: hub, Universe: uni, Risk: r, Execution: ex, Guardian: g,
		Ledger: led, Telegram: tg, CoinGlass: cg, Binance: bc,
		exitMgr: &exit.Manager{Execution: ex},
		state: model.StateIdle, cgScores: make(map[string]float64),
		lastWSHealthy: time.Now(),
	}
}

func (rt *Runtime) State() model.BotState {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.state
}

func (rt *Runtime) OpenPosition() *guardian.InternalPosition {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.openPos
}

func (rt *Runtime) CoinGlassScore(symbol string) float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.cgScores[symbol]
}

func (rt *Runtime) SetState(ctx context.Context, next model.BotState, reason string) {
	rt.mu.Lock()
	prev := rt.state
	rt.state = next
	runID := rt.runID
	rt.mu.Unlock()
	if rt.Ledger != nil && prev != next {
		_ = rt.Ledger.LogState(ctx, runID, string(prev), string(next), reason)
	}
}

func (rt *Runtime) Start(ctx context.Context) {
	rt.SetState(ctx, model.StateScanning, "engine_start")
	go rt.loop(ctx)
	go rt.guardianLoop(ctx)
	go rt.coinGlassLoop(ctx)
	go rt.universeLoop(ctx)
}

func (rt *Runtime) loop(ctx context.Context) {
	tick := time.NewTicker(config.EngineTick)
	flowCut := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	defer flowCut.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-flowCut.C:
			rt.Hub.ResetFlowOlderThan(time.Now().Add(-config.CVDWindow))
		case <-tick.C:
			rt.scan(ctx)
		}
	}
}

func (rt *Runtime) universeLoop(ctx context.Context) {
	t := time.NewTicker(config.UniverseRefresh)
	defer t.Stop()
	rt.refreshUniverse(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rt.refreshUniverse(ctx)
		}
	}
}

func (rt *Runtime) refreshUniverse(ctx context.Context) {
	if rt.Universe == nil || rt.Binance == nil {
		return
	}
	_, err := rt.Universe.Refresh(ctx)
	if err != nil {
		log.Printf("universe refresh: %v", err)
		rt.Risk.SetMarketHealthy(false)
		return
	}
	rt.Risk.SetMarketHealthy(true)
	rt.lastWSHealthy = time.Now()
	if rt.OnUniverseChanged != nil {
		rt.OnUniverseChanged(rt.Universe.ActiveSymbols())
	}
}

func (rt *Runtime) coinGlassLoop(ctx context.Context) {
	t := time.NewTicker(config.CoinGlassPoll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, sym := range rt.Universe.ActiveSymbols() {
				coin := trimUSDT(sym)
				sc, err := rt.CoinGlass.ScoreSymbol(ctx, coin)
				if err != nil {
					continue
				}
				rt.mu.Lock()
				rt.cgScores[sym] = sc.Score
				rt.mu.Unlock()
			}
		}
	}
}

func (rt *Runtime) guardianLoop(ctx context.Context) {
	t := time.NewTicker(config.GuardianInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rt.mu.RLock()
			pos := rt.openPos
			rt.mu.RUnlock()
			if pos == nil {
				rt.Risk.SetOpenPositions(0)
				continue
			}
			rt.Risk.SetOpenPositions(1)
			res := rt.Guardian.Verify(ctx, pos)
			if !res.OK {
				_ = rt.Ledger.InsertRiskEvent(ctx, "critical", res.Details, pos.Symbol, res.Details, res.Action)
				_ = rt.Telegram.Send(ctx, "GUARDIAN_"+res.Action, pos.Symbol)
				_ = rt.Guardian.HandleFailure(ctx, pos, res)
				rt.Risk.Kill()
				rt.SetState(ctx, model.StateKillSwitch, res.Details)
			}
		}
	}
}

func (rt *Runtime) scan(ctx context.Context) {
	now := time.Now().UTC()
	rt.Risk.ResetDailyIfNeeded(now)
	out := rt.rankSignals(now)
	rt.mu.Lock()
	rt.lastRank = out
	rt.mu.Unlock()
	rt.recordScan(out)

	rt.mu.RLock()
	pos := rt.openPos
	rt.mu.RUnlock()
	if pos != nil {
		rt.managePosition(ctx, pos)
		return
	}
	if rt.State() == model.StateKillSwitch || rt.State() == model.StatePaused {
		return
	}
	ok, _ := rt.Risk.AllowNewEntry(ctx)
	if !ok {
		return
	}
	for _, sig := range out.Signals {
		if !sig.WillFire {
			continue
		}
		rt.tryEnterSignal(ctx, sig)
		return
	}
}

func (rt *Runtime) rankSignals(now time.Time) signal.RankOutput {
	return rt.RankSignalsAt(now)
}

func (rt *Runtime) RankSignalsAt(now time.Time) signal.RankOutput {
	btc := rt.Hub.BTC5mChangePct()
	snap := rt.Risk.Get()
	live := config.Live.Get()
	armed := snap.TradingEnabled && !snap.Paused && !snap.KillSwitch
	canTrade := armed && snap.OpenPositions < live.MaxOpenPositions && snap.TradesToday < live.MaxTradesPerDay
	minTrades := live.MinTradesPerDay
	if minTrades <= 0 {
		minTrades = config.MinTradesPerDay
	}
	rt.mu.RLock()
	inPos := rt.openPos != nil
	rt.mu.RUnlock()
	riskOK, _ := rt.Risk.AllowNewEntry(context.Background())

	var inputs []signal.SymbolInput
	for _, sym := range rt.Universe.ActiveSymbols() {
		st, ok := rt.Hub.Snapshot(sym)
		if !ok {
			continue
		}
		rt.mu.RLock()
		cg := rt.cgScores[sym]
		rt.mu.RUnlock()
		inputs = append(inputs, signal.SymbolInput{
			Symbol: sym, State: st, CoinGlassScore: cg,
			BTCChange5mPct: btc, IsCore: isCoreSymbol(sym),
		})
	}
	return signal.Rank(signal.RankInput{
		Now: now, Symbols: inputs, TradesToday: snap.TradesToday,
		MinTradesPerDay: minTrades, Armed: armed, CanTrade: canTrade,
		InPosition: inPos, TradingEnabled: snap.TradingEnabled,
		Paused: snap.Paused, KillSwitch: snap.KillSwitch, RiskOK: riskOK,
		BotState: string(rt.State()), MarketHealthy: snap.MarketDataHealthy,
	})
}

func (rt *Runtime) ExecuteSignal(ctx context.Context, symbol string) error {
	out := rt.RankSignalsAt(time.Now().UTC())
	var target *model.ProSignal
	for i := range out.Universe {
		if out.Universe[i].Symbol == symbol {
			target = &out.Universe[i]
			break
		}
	}
	if target == nil {
		rt.recordExecute(symbol, false, "symbol not in universe")
		return fmt.Errorf("symbol not in universe")
	}
	if !target.CanExecute {
		rt.recordExecute(symbol, false, target.BlockReason)
		return fmt.Errorf("blocked: %s", target.BlockReason)
	}
	rt.mu.RLock()
	hasPos := rt.openPos != nil
	rt.mu.RUnlock()
	if hasPos {
		rt.recordExecute(symbol, false, "already in position")
		return fmt.Errorf("already in position")
	}
	rt.tryEnterSignal(ctx, *target)
	rt.recordExecute(symbol, true, fmt.Sprintf("manual entry %s %s", target.Playbook, target.Side))
	return nil
}

func isCoreSymbol(sym string) bool {
	for _, c := range config.AlwaysInclude {
		if c == sym {
			return true
		}
	}
	return false
}

func (rt *Runtime) managePosition(ctx context.Context, pos *guardian.InternalPosition) {
	st, ok := rt.Hub.Snapshot(pos.Symbol)
	if !ok {
		return
	}
	atr := pos.ATRAtEntry
	if atr <= 0 {
		atr = market.ATR(st.Candles5m, 14)
	}
	riskUSD := pos.RiskUSD
	if riskUSD <= 0 {
		riskUSD = config.Live.Get().RiskPerTradeUSD
	}
	act, ok := rt.exitMgr.Evaluate(exit.TickInput{
		Pos: pos, Mark: st.LastPrice, ATR: atr, RiskUSD: riskUSD,
		CVDFlip: exit.CVDFlipAgainst(st, pos.Side), StaleHrs: exit.HoldHours(pos),
	})
	if !ok {
		return
	}
	if err := rt.exitMgr.Apply(ctx, pos, act); err != nil {
		log.Printf("exit apply: %v", err)
		return
	}
	if act.ClosePct >= 1 || pos.RemainingQty <= 0 {
		rt.mu.Lock()
		rt.openPos = nil
		rt.mu.Unlock()
		rt.Risk.SetOpenPositions(0)
		rt.SetState(ctx, model.StateScanning, act.Reason)
		_ = rt.Telegram.Send(ctx, "EXIT_"+act.Reason, pos.Symbol)
	}
}

func (rt *Runtime) tryEnterSignal(ctx context.Context, sig model.ProSignal) {
	symbol := sig.Symbol
	side := sig.Side
	rt.SetState(ctx, model.StateSetupFound, sig.Playbook)
	rt.SetState(ctx, model.StateRiskChecking, "ok")
	rt.SetState(ctx, model.StateOrderPlacing, symbol)

	st, _ := rt.Hub.Snapshot(symbol)
	entry := st.LastPrice
	if entry <= 0 {
		rt.SetState(ctx, model.StateScanning, "no_price")
		return
	}
	cfg := config.Live.Get()
	sess := signal.SessionAdjustments(time.Now().UTC(), signal.CurrentSession(time.Now().UTC()))
	riskUSD := signal.RiskUSDForStrength(sig.Strength, sess)
	atr := sig.Extra.ATR
	stopDist := atr * 1.5
	if stopDist <= 0 {
		stopDist = entry * 0.005
	}
	var stop float64
	if side == model.SideLong {
		stop = entry - stopDist
	} else {
		stop = entry + stopDist
	}
	qty := execution.RiskQuantity(cfg.ActiveCapitalUSD, riskUSD, entry, stop, cfg.MaxLeverage)
	if qty <= 0 {
		_ = rt.Ledger.InsertMissedTrade(ctx, symbol, string(side), float64(sig.Strength)/100, "invalid_qty")
		rt.SetState(ctx, model.StateScanning, "invalid_qty")
		return
	}
	limitPx := entry
	if side == model.SideLong {
		limitPx = entry * 0.9999
	} else {
		limitPx = entry * 1.0001
	}

	rt.SetState(ctx, model.StateEntryPending, symbol)
	result, err := rt.Execution.PlacePostOnlyEntry(ctx, execution.EntryRequest{
		Symbol: symbol, Side: side, Quantity: qty, LimitPrice: limitPx,
	})
	if err != nil {
		_ = rt.Ledger.InsertMissedTrade(ctx, symbol, string(side), float64(sig.Strength)/100, err.Error())
		_ = rt.Telegram.Send(ctx, "ENTRY_MISSED", symbol+": "+err.Error())
		rt.SetState(ctx, model.StateScanning, "entry_failed")
		return
	}

	rt.SetState(ctx, model.StateEntryFilled, symbol)
	rt.SetState(ctx, model.StateStopPlacing, symbol)
	sl, err := rt.Execution.PlaceStopMarket(ctx, symbol, side, result.FilledQty, stop)
	if err != nil {
		_ = rt.Telegram.Send(ctx, "STOP_PLACEMENT_FAILED", symbol)
		_ = rt.Execution.EmergencyClose(ctx, symbol, side, result.FilledQty)
		rt.Risk.Pause()
		_ = rt.Ledger.InsertRiskEvent(ctx, "critical", "stop_failed", symbol, err.Error(), "emergency_exit")
		rt.SetState(ctx, model.StatePaused, "stop_failed")
		return
	}
	tp := execution.TakeProfitPrice(result.FilledPx, side, 2.5, riskUSD, result.FilledQty)
	_, _ = rt.Execution.PlaceTakeProfit(ctx, symbol, side, result.FilledQty, tp)

	rt.mu.Lock()
	rt.openPos = &guardian.InternalPosition{
		ID: 1, Symbol: symbol, Side: side,
		Quantity: result.FilledQty, RemainingQty: result.FilledQty,
		EntryPrice: result.FilledPx, StopPrice: stop, TakeProfitPrice: tp,
		HasStop: sl > 0, StopOrderID: sl, EntryTime: time.Now().UnixMilli(),
		Playbook: sig.Playbook, StrengthAtEntry: sig.Strength, Session: sig.Session,
		ExitPhase: "PROTECTED", ATRAtEntry: atr, RiskUSD: riskUSD,
	}
	rt.mu.Unlock()
	rt.Risk.IncTradesToday()
	_ = rt.Telegram.Send(ctx, "ENTRY_FILLED", symbol+" "+sig.Playbook)
	rt.SetState(ctx, model.StateInPosition, symbol)
}

func trimUSDT(s string) string {
	if len(s) > 4 && s[len(s)-4:] == "USDT" {
		return s[:len(s)-4]
	}
	return s
}
