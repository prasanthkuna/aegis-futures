package exit

import (
	"context"
	"time"

	"encore.app/execution"
	"encore.app/guardian"
	"encore.app/market"
	"encore.app/model"
)

type Manager struct {
	Execution *execution.Service
}

type TickInput struct {
	Pos       *guardian.InternalPosition
	Mark      float64
	ATR       float64
	RiskUSD   float64
	CVDFlip   bool
	StaleHrs  float64
}

type Action struct {
	ClosePct   float64 // 0-1, 1 = full close
	NewStop    float64
	Reason     string
	Phase      string
	UpdateStop bool
}

func (m *Manager) Evaluate(in TickInput) (Action, bool) {
	if in.Pos == nil || in.Mark <= 0 || in.RiskUSD <= 0 {
		return Action{}, false
	}
	r := rMultiple(in.Pos, in.Mark, in.RiskUSD)
	act := Action{Phase: in.Pos.ExitPhase}
	if act.Phase == "" {
		act.Phase = "PROTECTED"
	}

	switch {
	case r >= 2.5:
		return Action{ClosePct: 1, Reason: "full_tp_2.5r", Phase: "EXIT"}, true
	case in.StaleHrs >= 72 && r < 0.2:
		return Action{ClosePct: 1, Reason: "stale_72h", Phase: "EXIT"}, true
	case in.CVDFlip && r < 0.5:
		return Action{ClosePct: 1, Reason: "momentum_fade", Phase: "EXIT"}, true
	case r >= 1.0 && in.Pos.PartialPct < 0.4:
		newStop := breakevenPlus(in.Pos, in.RiskUSD, 0.25)
		return Action{
			ClosePct: 0.4, Reason: "partial_1r", Phase: "PARTIAL_1",
			NewStop: newStop, UpdateStop: true,
		}, true
	case r >= 0.5 && in.Pos.ExitPhase == "PROTECTED":
		newStop := in.Pos.EntryPrice
		return Action{
			Reason: "breakeven_0.5r", Phase: "BREAKEVEN",
			NewStop: newStop, UpdateStop: true,
		}, true
	case r >= 1.0 && in.Pos.ExitPhase == "PARTIAL_1" && in.ATR > 0:
		trail := trailStop(in.Pos, in.Mark, in.ATR, 1.5)
		if trail > in.Pos.StopPrice && in.Pos.Side == model.SideLong {
			return Action{Reason: "trail", Phase: "TRAILING", NewStop: trail, UpdateStop: true}, true
		}
		if trail < in.Pos.StopPrice && in.Pos.Side == model.SideShort && trail > 0 {
			return Action{Reason: "trail", Phase: "TRAILING", NewStop: trail, UpdateStop: true}, true
		}
	}
	if in.Pos != nil && r > in.Pos.PeakR {
		in.Pos.PeakR = r
	}
	return Action{}, false
}

func (m *Manager) Apply(ctx context.Context, pos *guardian.InternalPosition, act Action) error {
	if m == nil || m.Execution == nil || pos == nil {
		return nil
	}
	if act.ClosePct > 0 {
		qty := pos.RemainingQty
		if qty <= 0 {
			qty = pos.Quantity
		}
		if act.ClosePct < 1 {
			qty = qty * act.ClosePct
		}
		if err := m.Execution.EmergencyClose(ctx, pos.Symbol, pos.Side, qty); err != nil {
			return err
		}
		if act.ClosePct < 1 {
			pos.RemainingQty = pos.Quantity * (1 - act.ClosePct)
			pos.PartialPct = act.ClosePct
		} else {
			pos.RemainingQty = 0
		}
	}
	if act.UpdateStop && act.NewStop > 0 {
		pos.StopPrice = act.NewStop
	}
	pos.ExitPhase = act.Phase
	return nil
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

func HoldHours(pos *guardian.InternalPosition) float64 {
	if pos == nil || pos.EntryTime <= 0 {
		return 0
	}
	return float64(time.Now().UnixMilli()-pos.EntryTime) / 3600000
}

func CVDFlipAgainst(st market.SymbolState, side model.Side) bool {
	buy, sell := st.TakerBuyVol, st.TakerSellVol
	if buy+sell <= 0 {
		return false
	}
	if side == model.SideLong && sell > buy*1.2 {
		return true
	}
	if side == model.SideShort && buy > sell*1.2 {
		return true
	}
	return false
}
