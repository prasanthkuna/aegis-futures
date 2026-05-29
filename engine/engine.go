package engine

import (
	"context"
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
	"encore.app/strategy"
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
	if rt.State() == model.StateKillSwitch || rt.State() == model.StatePaused {
		return
	}
	rt.Risk.ResetDailyIfNeeded(time.Now())
	rt.mu.RLock()
	hasPos := rt.openPos != nil
	rt.mu.RUnlock()
	if hasPos {
		return
	}
	ok, reason := rt.Risk.AllowNewEntry(ctx)
	if !ok {
		return
	}
	btcCh := rt.Hub.BTC5mChangePct()
	for _, sym := range rt.Universe.ActiveSymbols() {
		st, ok := rt.Hub.Snapshot(sym)
		if !ok {
			continue
		}
		rt.mu.RLock()
		cg := rt.cgScores[sym]
		rt.mu.RUnlock()
		res := strategy.Evaluate(strategy.Input{
			Symbol: sym, State: st, CoinGlassScore: cg, BTCChange5mPct: btcCh,
		})
		if rt.Ledger != nil {
			_ = rt.Ledger.InsertSetupScore(ctx, sym, res.TradeScore,
				res.VolumeComponent, res.CVDComponent, res.StructureComponent,
				res.ContextComponent, res.DepthComponent, res.SessionComponent,
				res.Decision, res.Reason, string(res.SideHint))
		}
		if res.Decision != "trade" {
			continue
		}
		rt.tryEnter(ctx, sym, res)
		return
	}
	_ = reason
}

func (rt *Runtime) tryEnter(ctx context.Context, symbol string, res strategy.Result) {
	rt.SetState(ctx, model.StateSetupFound, res.Reason)
	rt.SetState(ctx, model.StateRiskChecking, "ok")
	rt.SetState(ctx, model.StateOrderPlacing, symbol)

	st, _ := rt.Hub.Snapshot(symbol)
	entry := st.LastPrice
	if entry <= 0 {
		rt.SetState(ctx, model.StateScanning, "no_price")
		return
	}
	qty := execution.RiskQuantity(config.ActiveCapitalUSD, config.RiskPerTradeUSD, entry,
		entry*0.005, config.MaxLeverage) // provisional stop distance 0.5% for sizing
	stop := execution.StopPrice(entry, res.SideHint, config.RiskPerTradeUSD, qty)
	qty = execution.RiskQuantity(config.ActiveCapitalUSD, config.RiskPerTradeUSD, entry, stop, config.MaxLeverage)
	if qty <= 0 {
		_ = rt.Ledger.InsertMissedTrade(ctx, symbol, string(res.SideHint), res.TradeScore, "invalid_qty")
		rt.SetState(ctx, model.StateScanning, "invalid_qty")
		return
	}
	limitPx := entry
	if res.SideHint == model.SideLong {
		limitPx = entry * 0.9999
	} else {
		limitPx = entry * 1.0001
	}

	rt.SetState(ctx, model.StateEntryPending, symbol)
	result, err := rt.Execution.PlacePostOnlyEntry(ctx, execution.EntryRequest{
		Symbol: symbol, Side: res.SideHint, Quantity: qty, LimitPrice: limitPx,
	})
	if err != nil {
		_ = rt.Ledger.InsertMissedTrade(ctx, symbol, string(res.SideHint), res.TradeScore, err.Error())
		_ = rt.Telegram.Send(ctx, "ENTRY_MISSED", symbol+": "+err.Error())
		rt.SetState(ctx, model.StateScanning, "entry_failed")
		return
	}

	rt.SetState(ctx, model.StateEntryFilled, symbol)
	rt.SetState(ctx, model.StateStopPlacing, symbol)
	sl, err := rt.Execution.PlaceStopMarket(ctx, symbol, res.SideHint, result.FilledQty, stop)
	if err != nil {
		_ = rt.Telegram.Send(ctx, "STOP_PLACEMENT_FAILED", symbol)
		_ = rt.Execution.EmergencyClose(ctx, symbol, res.SideHint, result.FilledQty)
		rt.Risk.Pause()
		_ = rt.Ledger.InsertRiskEvent(ctx, "critical", "stop_failed", symbol, err.Error(), "emergency_exit")
		rt.SetState(ctx, model.StatePaused, "stop_failed")
		return
	}
	tp := execution.TakeProfitPrice(result.FilledPx, res.SideHint, 2.0, config.RiskPerTradeUSD, result.FilledQty)
	_, _ = rt.Execution.PlaceTakeProfit(ctx, symbol, res.SideHint, result.FilledQty, tp)

	rt.mu.Lock()
	rt.openPos = &guardian.InternalPosition{
		ID: 1, Symbol: symbol, Side: res.SideHint,
		Quantity: result.FilledQty, StopPrice: stop, HasStop: sl > 0,
	}
	rt.mu.Unlock()
	_ = rt.Telegram.Send(ctx, "ENTRY_FILLED", symbol)
	rt.SetState(ctx, model.StateInPosition, symbol)
}

func trimUSDT(s string) string {
	if len(s) > 4 && s[len(s)-4:] == "USDT" {
		return s[:len(s)-4]
	}
	return s
}
