package signal

import (
	"fmt"

	"encore.app/model"
)

func WeakestLink(c model.ScoreComponents) string {
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

func TierFor(strength, floor int) string {
	if strength >= floor {
		return "READY"
	}
	if strength >= floor-10 {
		return "BUILDING"
	}
	return "BELOW_FLOOR"
}

func BuildNarrative(universe []model.ProSignal, floor int, flatCVD int, n int) string {
	if len(universe) == 0 {
		return "no market data yet — waiting for WS candles"
	}
	above := 0
	triggered := 0
	var maxS int
	for _, s := range universe {
		if s.Strength >= floor {
			above++
		}
		if s.PlaybookTriggered {
			triggered++
		}
		if s.Strength > maxS {
			maxS = s.Strength
		}
	}
	return fmt.Sprintf(
		"%d/%d flat CVD · %d playbook triggers · %d above floor %d · peak strength %d",
		flatCVD, n, triggered, above, floor, maxS,
	)
}

func MedianStrength(signals []model.ProSignal) int {
	if len(signals) == 0 {
		return 0
	}
	vals := make([]int, len(signals))
	for i, s := range signals {
		vals[i] = s.Strength
	}
	for i := 1; i < len(vals); i++ {
		for j := i; j > 0 && vals[j-1] > vals[j]; j-- {
			vals[j], vals[j-1] = vals[j-1], vals[j]
		}
	}
	mid := len(vals) / 2
	if len(vals)%2 == 0 {
		return (vals[mid-1] + vals[mid]) / 2
	}
	return vals[mid]
}

func blockReason(in RankInput, strength, floor int, gatesOK bool) (bool, string) {
	if in.InPosition {
		return false, "in_position"
	}
	if in.KillSwitch {
		return false, "kill_switch"
	}
	if in.Paused {
		return false, "paused"
	}
	if !in.TradingEnabled {
		return false, "trading_disabled"
	}
	if !in.RiskOK {
		return false, "risk_limit"
	}
	if strength < floor {
		return false, fmt.Sprintf("below_floor_%d", floor-strength)
	}
	if !gatesOK {
		return false, "gates_blocked"
	}
	return true, ""
}
