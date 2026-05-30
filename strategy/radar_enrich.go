package strategy

import (
	"fmt"
	"strings"

	"encore.app/config"
	"encore.app/model"
)

func TierLabel(score, minScore, aplus float64, radarDecision string) string {
	if score >= aplus {
		return "A+"
	}
	if radarDecision == "trade" {
		return "trade"
	}
	if radarDecision == "watch" {
		return "watch"
	}
	return "skip"
}

func WeakestLink(res Result, reason string) string {
	switch {
	case strings.Contains(reason, "btc"):
		return "btc_block"
	case strings.Contains(reason, "spread"):
		return "spread"
	case strings.Contains(reason, "flow"):
		return "flow_flat"
	case strings.Contains(reason, "structure"):
		return "no_break"
	case strings.Contains(reason, "score"):
		return lowestComponentTag(res)
	default:
		return lowestComponentTag(res)
	}
}

func lowestComponentTag(res Result) string {
	type comp struct {
		tag string
		v   float64
	}
	comps := []comp{
		{"no_surge", res.VolumeComponent},
		{"flow_flat", res.CVDComponent},
		{"no_break", res.StructureComponent},
		{"context", res.ContextComponent},
		{"spread", res.DepthComponent},
		{"session", res.SessionComponent},
	}
	best := comps[0]
	for _, c := range comps[1:] {
		if c.v < best.v {
			best = c
		}
	}
	if best.v >= 0.85 {
		return "balanced"
	}
	return best.tag
}

func GateFlagsFor(res Result, minScore, btcPct, spreadBps float64) model.GateFlags {
	g := model.GateFlags{
		MinScore:  res.TradeScore >= minScore,
		Structure: res.StructureComponent >= 1,
		Spread:    spreadBps <= 12,
	}
	g.Flow = res.CVDComponent >= 0.35 && res.TakerFlow != "flat"
	g.Btc = true
	if res.SideHint == model.SideLong || (res.StructureComponent >= 1 && res.TakerFlow == "buy") {
		g.Btc = btcPct > config.BTCBlockLongPct
	} else if res.SideHint == model.SideShort || (res.StructureComponent >= 1 && res.TakerFlow == "sell") {
		g.Btc = btcPct < config.BTCBlockShortPct
	}
	return g
}

func BtcRegimeTag(btcPct float64) string {
	if btcPct <= config.BTCBlockLongPct {
		return "block_long"
	}
	if btcPct >= config.BTCBlockShortPct {
		return "block_short"
	}
	return "neutral"
}

func ComponentsFrom(res Result) model.ScoreComponents {
	return model.ScoreComponents{
		Volume: res.VolumeComponent, CVD: res.CVDComponent,
		Structure: res.StructureComponent, Context: res.ContextComponent,
		Depth: res.DepthComponent, Session: res.SessionComponent,
	}
}

func SideHintString(s model.Side) string {
	if s == "" {
		return ""
	}
	return string(s)
}

func BuildRegime(items []model.SymbolSnapshot, btcPct float64) model.RadarRegime {
	reg := model.RadarRegime{BtcChange5mPct: btcPct}
	var surges []float64
	for _, it := range items {
		switch it.Decision {
		case "trade":
			reg.TradeCount++
		case "watch":
			reg.WatchCount++
		default:
			reg.SkipCount++
		}
		if it.TradeScore > reg.MaxScore {
			reg.MaxScore = it.TradeScore
		}
		surges = append(surges, it.VolumeSurge)
	}
	reg.MedianSurge = median(surges)
	switch {
	case reg.TradeCount > 0:
		reg.Label = "MOMENTUM"
		reg.Summary = fmt.Sprintf("%d tradeable, max score %.2f", reg.TradeCount, reg.MaxScore)
	case reg.WatchCount > 0:
		reg.Label = "WATCH"
		reg.Summary = fmt.Sprintf("%d watch, max score %.2f", reg.WatchCount, reg.MaxScore)
	default:
		reg.Label = "CHOP"
		reg.Summary = fmt.Sprintf("0/%d tradeable, max score %.2f, median surge %.0f%%",
			len(items), reg.MaxScore, reg.MedianSurge*100)
	}
	return reg
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	cp := append([]float64(nil), vals...)
	for i := 0; i < len(cp); i++ {
		for j := i + 1; j < len(cp); j++ {
			if cp[j] < cp[i] {
				cp[i], cp[j] = cp[j], cp[i]
			}
		}
	}
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2
	}
	return cp[mid]
}
