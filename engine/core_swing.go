package engine

import (
	"context"
	"fmt"
	"time"

	"encore.app/config"
	"encore.app/execution"
	"encore.app/exit"
	"encore.app/guardian"
	"encore.app/market"
	"encore.app/model"
	"encore.app/signal"
)

func (rt *Runtime) scanCoreSwing(ctx context.Context) {
	now := time.Now().UTC()
	rt.Risk.ResetDailyIfNeeded(now)
	live := config.Live.Get()
	snap := rt.Risk.Get()
	armed := snap.TradingEnabled && !snap.Paused && !snap.KillSwitch
	canTrade := armed && snap.OpenPositions < live.MaxOpenPositions && snap.TradesToday < live.MaxTradesPerDay
	block := ""
	if !armed {
		block = "not_armed"
	} else if snap.TradesToday >= live.MaxTradesPerDay {
		block = "max_trades_day"
	} else if snap.OpenPositions >= live.MaxOpenPositions {
		block = "in_position"
	}
	riskOK, riskReason := rt.Risk.AllowNewEntry(ctx)
	if !riskOK {
		canTrade = false
		if block == "" {
			block = riskReason
		}
	}

	rt.mu.RLock()
	pos := rt.openPos
	rt.mu.RUnlock()
	if pos != nil {
		rt.manageCoreSwingPosition(ctx, pos)
		rt.publishCoreSwingRank(ctx, now, canTrade, block, nil)
		return
	}
	if rt.State() == model.StateKillSwitch || rt.State() == model.StatePaused {
		rt.publishCoreSwingRank(ctx, now, false, string(rt.State()), nil)
		return
	}

	var fired *model.ProSignal
	for _, sym := range config.CoreSwingSymbols() {
		h1, close1h, ok := rt.Hub.Candles1h(sym)
		if !ok {
			continue
		}
		rt.mu.Lock()
		last := rt.last1hClose[sym]
		if !close1h.After(last) {
			rt.mu.Unlock()
			continue
		}
		rt.last1hClose[sym] = close1h
		rt.mu.Unlock()

		side, triggered, pb := signal.EvalCoreSwing(sym, h1)
		if !triggered || side == "" {
			continue
		}
		st, _ := rt.Hub.Snapshot(sym)
		sig := signal.BuildCoreSwingSignal(sym, side, pb, st, canTrade, block)
		if fired == nil {
			s := sig
			fired = &s
		}
		if canTrade {
			rt.tryEnterCoreSwing(ctx, sig)
			rt.publishCoreSwingRank(ctx, now, canTrade, block, fired)
			return
		}
	}
	rt.publishCoreSwingRank(ctx, now, canTrade, block, fired)
}

func (rt *Runtime) publishCoreSwingRank(ctx context.Context, now time.Time, canTrade bool, block string, fired *model.ProSignal) {
	var universe, signals []model.ProSignal
	triggeredN := 0
	for _, sym := range config.CoreSwingSymbols() {
		st, ok := rt.Hub.Snapshot(sym)
		if !ok {
			continue
		}
		spec, ok := config.CoreSwingSpecFor(sym)
		if !ok {
			continue
		}
		var sig model.ProSignal
		if h1, _, ok := rt.Hub.Candles1h(sym); ok {
			side, triggered, pb := signal.EvalCoreSwing(sym, h1)
			if triggered {
				triggeredN++
				sig = signal.BuildCoreSwingSignal(sym, side, pb, st, canTrade, block)
			} else {
				sig = signal.BuildCoreSwingSignal(sym, "", spec.Playbook, st, false, "await_setup")
				sig.PlaybookTriggered = false
				sig.Strength = 40
				sig.Tier = "WAIT"
				sig.WillFire = false
				sig.CanExecute = false
			}
		} else {
			sig = signal.BuildCoreSwingSignal(sym, "", spec.Playbook, st, false, "no_1h")
			sig.PlaybookTriggered = false
			sig.Tier = "WAIT"
		}
		if fired != nil && fired.Symbol == sym {
			sig = *fired
		}
		universe = append(universe, sig)
		if sig.WillFire && sig.Side != "" {
			signals = append(signals, sig)
		}
	}
	live := config.Live.Get()
	out := signal.RankOutput{
		Universe: universe,
		Signals:  signals,
		Floor:    0,
		Session: model.SessionCockpit{
			Session: "CORE_1H", Floor: 0,
			TradesToday: rt.Risk.Get().TradesToday,
			MaxTradesPerDay: live.MaxTradesPerDay,
			MinTradesPerDay: 0,
			TargetTrades: live.TargetTradesPerDay,
			ActivePlaybooks: []string{"S4_SQUEEZE_LIBERAL", "S11_EMA_TREND_STRICT"},
			Armed: canTrade, TradingEnabled: rt.Risk.Get().TradingEnabled,
			RegimeLabel: "CORE_SWING",
			SignalCount: len(signals),
		},
		Regime: model.RadarRegime{
			Label: "CORE_SWING",
			Summary: fmt.Sprintf("%d/%d playbook triggers · paper=%v",
				triggeredN, len(config.CoreSwingSymbols()), config.IsPaperMode()),
			TradeCount: len(signals),
		},
		Heartbeat: model.EngineHeartbeat{
			LastScanAt: now, SymbolsScanned: len(universe),
			Candidates: triggeredN, WillFireCount: len(signals),
			MaxStrength: 100, MarketDataHealthy: true,
			BotState: string(rt.State()), UniverseSize: len(universe),
		},
		Narrative: fmt.Sprintf("Core swing 1h (%s) | trades %d/%d | risk $%.2f | auto-entry on 1h close",
			paperLiveLabel(), rt.Risk.Get().TradesToday, live.MaxTradesPerDay, live.RiskPerTradeUSD),
	}
	rt.mu.Lock()
	rt.lastRank = out
	rt.mu.Unlock()
	rt.recordScan(out)
	_ = ctx
	_ = now
}

func paperLiveLabel() string {
	if config.IsPaperMode() {
		return "paper"
	}
	return "live"
}

func (rt *Runtime) tryEnterCoreSwing(ctx context.Context, sig model.ProSignal) {
	spec, ok := config.CoreSwingSpecFor(sig.Symbol)
	if !ok {
		return
	}
	side := sig.Side
	rt.SetState(ctx, model.StateSetupFound, sig.Playbook)
	rt.SetState(ctx, model.StateRiskChecking, "ok")
	rt.SetState(ctx, model.StateOrderPlacing, sig.Symbol)

	st, _ := rt.Hub.Snapshot(sig.Symbol)
	entry := st.LastPrice
	if entry <= 0 {
		rt.SetState(ctx, model.StateScanning, "no_price")
		return
	}
	h1, _, ok := rt.Hub.Candles1h(sig.Symbol)
	if !ok {
		rt.SetState(ctx, model.StateScanning, "no_1h")
		return
	}
	atr := market.ATR(h1, 14)
	cfg := config.Live.Get()
	riskUSD := cfg.RiskPerTradeUSD
	stopDist := exit.ApplyCoreSwingStopDist(entry, atr, spec.StopATR)
	var stop float64
	if side == model.SideLong {
		stop = entry - stopDist
	} else {
		stop = entry + stopDist
	}
	qty := execution.RiskQuantity(cfg.ActiveCapitalUSD, riskUSD, entry, stop, cfg.MaxLeverage)
	if qty <= 0 {
		rt.SetState(ctx, model.StateScanning, "invalid_qty")
		return
	}
	tp := execution.TakeProfitPrice(entry, side, spec.TargetRR, riskUSD, qty)

	if config.IsPaperMode() {
		rt.SetState(ctx, model.StateEntryPending, sig.Symbol)
		rt.SetState(ctx, model.StateEntryFilled, sig.Symbol)
		pos := &guardian.InternalPosition{
			ID: 1, Symbol: sig.Symbol, Side: side,
			Quantity: qty, RemainingQty: qty,
			EntryPrice: entry, StopPrice: stop, TakeProfitPrice: tp,
			HasStop: true, EntryTime: time.Now().UnixMilli(),
			Playbook: sig.Playbook, StrengthAtEntry: 100, Session: "CORE_1H",
			ExitPhase: "PROTECTED", ATRAtEntry: atr, RiskUSD: riskUSD,
			TargetRR: spec.TargetRR, MaxHoldHours: float64(config.CoreSwingMaxHoldHrs),
			Paper: true,
		}
		rt.mu.Lock()
		rt.openPos = pos
		rt.mu.Unlock()
		rt.Risk.IncTradesToday()
		rt.Risk.SetOpenPositions(1)
		_ = rt.Telegram.Send(ctx, "PAPER_ENTRY", fmt.Sprintf("%s %s rr%.1f stopATR%.1f $%.2f risk",
			sig.Symbol, sig.Playbook, spec.TargetRR, spec.StopATR, riskUSD))
		rt.SetState(ctx, model.StateInPosition, sig.Symbol)
		return
	}

	limitPx := entry
	if side == model.SideLong {
		limitPx = entry * 0.9999
	} else {
		limitPx = entry * 1.0001
	}

	rt.SetState(ctx, model.StateEntryPending, sig.Symbol)
	result, err := rt.Execution.PlacePostOnlyEntry(ctx, execution.EntryRequest{
		Symbol: sig.Symbol, Side: side, Quantity: qty, LimitPrice: limitPx,
	})
	if err != nil {
		_ = rt.Telegram.Send(ctx, "ENTRY_MISSED", sig.Symbol+": "+err.Error())
		rt.SetState(ctx, model.StateScanning, "entry_failed")
		return
	}

	rt.SetState(ctx, model.StateEntryFilled, sig.Symbol)
	rt.SetState(ctx, model.StateStopPlacing, sig.Symbol)
	sl, err := rt.Execution.PlaceStopMarket(ctx, sig.Symbol, side, result.FilledQty, stop)
	if err != nil {
		_ = rt.Telegram.Send(ctx, "STOP_PLACEMENT_FAILED", sig.Symbol)
		_ = rt.Execution.EmergencyClose(ctx, sig.Symbol, side, result.FilledQty)
		rt.Risk.Pause()
		rt.SetState(ctx, model.StatePaused, "stop_failed")
		return
	}
	_, _ = rt.Execution.PlaceTakeProfit(ctx, sig.Symbol, side, result.FilledQty, tp)

	pos := &guardian.InternalPosition{
		ID: 1, Symbol: sig.Symbol, Side: side,
		Quantity: result.FilledQty, RemainingQty: result.FilledQty,
		EntryPrice: result.FilledPx, StopPrice: stop, TakeProfitPrice: tp,
		HasStop: sl > 0, StopOrderID: sl, EntryTime: time.Now().UnixMilli(),
		Playbook: sig.Playbook, StrengthAtEntry: 100, Session: "CORE_1H",
		ExitPhase: "PROTECTED", ATRAtEntry: atr, RiskUSD: riskUSD,
		TargetRR: spec.TargetRR, MaxHoldHours: float64(config.CoreSwingMaxHoldHrs),
	}
	rt.mu.Lock()
	rt.openPos = pos
	rt.mu.Unlock()
	rt.Risk.IncTradesToday()
	_ = rt.Telegram.Send(ctx, "CORE_ENTRY", fmt.Sprintf("%s %s rr%.1f stopATR%.1f $%.2f risk",
		sig.Symbol, sig.Playbook, spec.TargetRR, spec.StopATR, riskUSD))
	rt.SetState(ctx, model.StateInPosition, sig.Symbol)
}

func (rt *Runtime) manageCoreSwingPosition(ctx context.Context, pos *guardian.InternalPosition) {
	st, ok := rt.Hub.Snapshot(pos.Symbol)
	if !ok {
		return
	}
	atr := pos.ATRAtEntry
	if atr <= 0 {
		if h1, _, ok := rt.Hub.Candles1h(pos.Symbol); ok {
			atr = market.ATR(h1, 14)
		}
	}
	riskUSD := pos.RiskUSD
	if riskUSD <= 0 {
		riskUSD = config.Live.Get().RiskPerTradeUSD
	}
	act, ok := exit.EvaluateCoreSwing(exit.TickInput{
		Pos: pos, Mark: st.LastPrice, ATR: atr, RiskUSD: riskUSD,
		StaleHrs: exit.HoldHours(pos),
	})
	if !ok {
		return
	}
	if pos.Paper {
		rt.closePaperCoreSwing(ctx, pos, st.LastPrice, act)
		return
	}
	if err := rt.exitMgr.Apply(ctx, pos, act); err != nil {
		return
	}
	if act.ClosePct >= 1 || pos.RemainingQty <= 0 {
		rt.mu.Lock()
		rt.openPos = nil
		rt.mu.Unlock()
		rt.Risk.SetOpenPositions(0)
		rt.SetState(ctx, model.StateScanning, act.Reason)
		_ = rt.Telegram.Send(ctx, "CORE_EXIT_"+act.Reason, pos.Symbol)
	}
}

func (rt *Runtime) closePaperCoreSwing(ctx context.Context, pos *guardian.InternalPosition, mark float64, act exit.Action) {
	if rt.paper == nil {
		return
	}
	t := rt.paper.Close(pos, mark, act.Reason)
	rt.Risk.RecordClosePnL(t.NetPnL)
	rt.mu.Lock()
	rt.openPos = nil
	rt.mu.Unlock()
	rt.Risk.SetOpenPositions(0)
	rt.SetState(ctx, model.StateScanning, act.Reason)
	_ = rt.Telegram.Send(ctx, "PAPER_EXIT_"+act.Reason,
		fmt.Sprintf("%s net $%.2f (%.2fR) fees $%.2f", pos.Symbol, t.NetPnL, t.RMultiple, t.Fees))
}
