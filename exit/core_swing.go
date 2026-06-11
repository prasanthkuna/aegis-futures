package exit

import (
	"encore.app/config"
	"encore.app/guardian"
	"encore.app/model"
)

// EvaluateCoreSwing uses fixed RR target + max hold for core swing playbooks.
func EvaluateCoreSwing(in TickInput) (Action, bool) {
	if in.Pos == nil || in.Mark <= 0 || in.RiskUSD <= 0 {
		return Action{}, false
	}
	rr := in.Pos.TargetRR
	if rr <= 0 {
		rr = 4.0
	}
	maxHrs := in.Pos.MaxHoldHours
	if maxHrs <= 0 {
		maxHrs = config.CoreSwingMaxHoldHrs
	}
	r := rMultiple(in.Pos, in.Mark, in.RiskUSD)

	if r >= rr {
		return Action{ClosePct: 1, Reason: "core_tp_rr", Phase: "EXIT"}, true
	}
	if in.StaleHrs >= maxHrs {
		return Action{ClosePct: 1, Reason: "core_time_stop", Phase: "EXIT"}, true
	}
	hitStop := (in.Pos.Side == model.SideLong && in.Mark <= in.Pos.StopPrice) ||
		(in.Pos.Side == model.SideShort && in.Mark >= in.Pos.StopPrice)
	if hitStop {
		return Action{ClosePct: 1, Reason: "core_stop", Phase: "EXIT"}, true
	}
	return Action{}, false
}

// ApplyCoreSwingStopDist returns stop distance from ATR multiplier.
func ApplyCoreSwingStopDist(entry, atr, stopATR float64) float64 {
	if atr <= 0 {
		return entry * 0.005
	}
	d := atr * stopATR
	minD := entry * 0.0015
	maxD := entry * 0.025
	if d < minD {
		d = minD
	}
	if d > maxD {
		d = maxD
	}
	return d
}

// StampCoreSwingPos sets exit params on a new position.
func StampCoreSwingPos(pos *guardian.InternalPosition, spec config.CoreSwingSpec) {
	if pos == nil {
		return
	}
	pos.TargetRR = spec.TargetRR
	pos.MaxHoldHours = float64(config.CoreSwingMaxHoldHrs)
}
