package backtest

import (
	"encore.app/exit"
	"encore.app/guardian"
	"encore.app/market"
	"encore.app/model"
	"time"
)

func holdHours(pos *guardian.InternalPosition, now time.Time) float64 {
	if pos == nil || pos.EntryTime <= 0 {
		return 0
	}
	return now.Sub(time.UnixMilli(pos.EntryTime)).Hours()
}

func evaluateExit(pos *guardian.InternalPosition, st market.SymbolState, mark, atr, riskUSD float64, now time.Time, p ExitParams) (exit.Action, bool) {
	if pos == nil || mark <= 0 || riskUSD <= 0 {
		return exit.Action{}, false
	}
	r := rMultiple(pos, mark, riskUSD)
	phase := pos.ExitPhase
	if phase == "" {
		phase = "PROTECTED"
	}
	switch {
	case r >= p.FullTPAtR:
		return exit.Action{ClosePct: 1, Reason: "full_tp", Phase: "EXIT"}, true
	case holdHours(pos, now) >= p.StaleHours && r < 0.2:
		return exit.Action{ClosePct: 1, Reason: "stale", Phase: "EXIT"}, true
	case exit.CVDFlipAgainst(st, pos.Side) && r < 0.5:
		return exit.Action{ClosePct: 1, Reason: "momentum_fade", Phase: "EXIT"}, true
	case r >= p.PartialAtR && pos.PartialPct < p.PartialPct:
		newStop := breakevenPlus(pos, riskUSD, 0.25)
		return exit.Action{
			ClosePct: p.PartialPct, Reason: "partial", Phase: "PARTIAL_1",
			NewStop: newStop, UpdateStop: true,
		}, true
	case r >= p.BEAtR && phase == "PROTECTED":
		return exit.Action{
			Reason: "breakeven", Phase: "BREAKEVEN",
			NewStop: pos.EntryPrice, UpdateStop: true,
		}, true
	case r >= p.PartialAtR && phase == "PARTIAL_1" && atr > 0:
		trail := trailStop(pos, mark, atr, p.TrailATR)
		if pos.Side == model.SideLong && trail > pos.StopPrice {
			return exit.Action{Reason: "trail", Phase: "TRAILING", NewStop: trail, UpdateStop: true}, true
		}
		if pos.Side == model.SideShort && trail < pos.StopPrice && trail > 0 {
			return exit.Action{Reason: "trail", Phase: "TRAILING", NewStop: trail, UpdateStop: true}, true
		}
	}
	if r > pos.PeakR {
		pos.PeakR = r
	}
	return exit.Action{}, false
}

func rMultiple(pos *guardian.InternalPosition, mark, riskUSD float64) float64 {
	if pos == nil || riskUSD <= 0 {
		return 0
	}
	var pnl float64
	if pos.Side == model.SideLong {
		pnl = (mark - pos.EntryPrice) * pos.Quantity
	} else {
		pnl = (pos.EntryPrice - mark) * pos.Quantity
	}
	return pnl / riskUSD
}

func breakevenPlus(pos *guardian.InternalPosition, riskUSD, rOff float64) float64 {
	dist := riskUSD / pos.Quantity * rOff
	if pos.Side == model.SideLong {
		return pos.EntryPrice + dist
	}
	return pos.EntryPrice - dist
}

func trailStop(pos *guardian.InternalPosition, mark, atr, mult float64) float64 {
	if pos.Side == model.SideLong {
		return mark - atr*mult
	}
	return mark + atr*mult
}

func stopHit(pos *guardian.InternalPosition, b Bar) (hit bool, px float64) {
	if pos == nil || pos.StopPrice <= 0 {
		return false, 0
	}
	if pos.Side == model.SideLong && b.Low <= pos.StopPrice {
		return true, pos.StopPrice
	}
	if pos.Side == model.SideShort && b.High >= pos.StopPrice {
		return true, pos.StopPrice
	}
	return false, 0
}

func applyExitAction(pos *guardian.InternalPosition, act exit.Action) {
	if act.ClosePct > 0 {
		if act.ClosePct >= 1 {
			pos.RemainingQty = 0
		} else {
			pos.RemainingQty = pos.Quantity * (1 - act.ClosePct)
			pos.PartialPct = act.ClosePct
		}
	}
	if act.UpdateStop && act.NewStop > 0 {
		pos.StopPrice = act.NewStop
	}
	if act.Phase != "" {
		pos.ExitPhase = act.Phase
	}
}
