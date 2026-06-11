package signal

import "time"

type SessionProfile struct {
	Name            string
	Floor           int
	MaxSpreadBps    float64
	SizeMult        float64
	Playbooks       []string
	VolW, CvdW      float64
	StructW, VwapW  float64
	DepthW, SessW   float64
}

func CurrentSession(now time.Time) SessionProfile {
	h := now.UTC().Hour()
	switch {
	case h >= 0 && h < 8:
		return SessionProfile{
			Name: "asia", Floor: 55, MaxSpreadBps: 10, SizeMult: 0.85,
			Playbooks: []string{"SESSION_BREAKOUT", "MEAN_REVERT_VWAP", "VOL_CLIMAX_FADE", "FORCED_FLOW_FADE"},
			VolW: 0.20, CvdW: 0.25, StructW: 0.15, VwapW: 0.40, DepthW: 0.05, SessW: 0.05,
		}
	case h >= 8 && h < 14:
		return SessionProfile{
			Name: "london", Floor: 58, MaxSpreadBps: 8, SizeMult: 1.0,
			Playbooks: []string{"MOMENTUM_BURST", "SESSION_BREAKOUT", "FORCED_FLOW_FADE"},
			VolW: 0.30, CvdW: 0.30, StructW: 0.35, VwapW: 0.05, DepthW: 0.05, SessW: 0.05,
		}
	case h >= 14 && h < 22:
		return SessionProfile{
			Name: "us", Floor: 58, MaxSpreadBps: 8, SizeMult: 1.0,
			Playbooks: []string{"MOMENTUM_BURST", "SESSION_BREAKOUT", "FORCED_FLOW_FADE"},
			VolW: 0.35, CvdW: 0.35, StructW: 0.25, VwapW: 0.05, DepthW: 0.05, SessW: 0.05,
		}
	default:
		return SessionProfile{
			Name: "late_us", Floor: 52, MaxSpreadBps: 12, SizeMult: 0.9,
			Playbooks: []string{"MEAN_REVERT_VWAP", "VOL_CLIMAX_FADE", "MOMENTUM_BURST", "FORCED_FLOW_FADE"},
			VolW: 0.25, CvdW: 0.25, StructW: 0.20, VwapW: 0.30, DepthW: 0.05, SessW: 0.05,
		}
	}
}

func SessionAdjustments(now time.Time, base SessionProfile) SessionProfile {
	wd := now.UTC().Weekday()
	h := now.UTC().Hour()
	if wd == time.Friday && h >= 18 {
		base.Floor += 5
		base.SizeMult *= 0.75
	}
	if wd == time.Saturday || wd == time.Sunday {
		base.Floor += 8
		base.SizeMult *= 0.75
	}
	return base
}

func InFundingWindow(now time.Time) bool {
	h, m := now.UTC().Hour(), now.UTC().Minute()
	for _, fh := range []int{0, 8, 16} {
		diff := (h-fh)*60 + m
		if diff >= -15 && diff <= 15 {
			return true
		}
	}
	return false
}

func NextFloorDropUTC(now time.Time, tradesToday, minTrades int) string {
	if tradesToday >= minTrades {
		return ""
	}
	h := now.UTC().Hour()
	switch {
	case h < 12:
		return "12:00 UTC"
	case h < 15:
		return "15:00 UTC"
	case h < 18:
		return "18:00 UTC"
	case h < 21:
		return "21:00 UTC"
	default:
		return ""
	}
}

func AdaptiveFloor(sess SessionProfile, now time.Time, tradesToday, minTrades int) int {
	floor := sess.Floor
	if tradesToday >= minTrades {
		return floor
	}
	h := now.UTC().Hour()
	if h >= 12 {
		floor -= 5
	}
	if h >= 15 {
		floor -= 3
	}
	if h >= 18 {
		floor -= 2
	}
	if h >= 21 {
		floor -= 5
	}
	if floor < 45 {
		floor = 45
	}
	return floor
}
