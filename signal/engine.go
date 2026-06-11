package signal

import (
	"fmt"
	"sort"
	"time"

	"encore.app/config"
	"encore.app/market"
	"encore.app/model"
	"encore.app/strategy"
)

type SymbolInput struct {
	Symbol         string
	State          market.SymbolState
	CoinGlassScore float64
	BTCChange5mPct float64
	IsCore         bool
}

type RankInput struct {
	Now            time.Time
	Symbols        []SymbolInput
	TradesToday    int
	MinTradesPerDay int
	Armed          bool
	CanTrade       bool
	InPosition     bool
}

type RankOutput struct {
	Signals  []model.ProSignal
	Session  model.SessionCockpit
	Floor    int
	Regime   model.RadarRegime
}

func Rank(in RankInput) RankOutput {
	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sess := SessionAdjustments(now, CurrentSession(now))
	floor := AdaptiveFloor(sess, now, in.TradesToday, in.MinTradesPerDay)
	chop := true

	var candidates []model.ProSignal
	for _, sym := range in.Symbols {
		st := sym.State
		vol := volumeSurge(st.Candles5m)
		cvd, flow, cvdState := cvdMetrics(st)
		_, brokeHi, brokeLo := structureBreak(st.Candles5m)
		ctx := clamp01((sym.CoinGlassScore + 1) / 2)
		depth := depthScore(st.SpreadBps)
		vwap := market.VWAP(st.Candles5m)
		atr := market.ATR(st.Candles5m, 14)
		ema9 := market.EMA(st.Candles5m, 9)
		vwapDev := market.VWAPDeviation(st.LastPrice, vwap)

		pbResults := []PlaybookResult{
			evalMomentum(st, sym.BTCChange5mPct),
			evalSessionBreakout(st),
			evalMeanRevert(st, chop),
		}
		best := PlaybookResult{Score: 0}
		for _, pb := range pbResults {
			if !playbookAllowed(pb.ID, sess) {
				continue
			}
			if pb.Triggered && pb.Score > best.Score {
				best = pb
			} else if !best.Triggered && pb.Score > best.Score {
				best = pb
			}
		}
		if !best.Triggered && best.Score < 0.15 {
			continue
		}
		side := best.Side
		if side == "" {
			if brokeHi {
				side = model.SideLong
			} else if brokeLo {
				side = model.SideShort
			} else if flow == "buy" {
				side = model.SideLong
			} else if flow == "sell" {
				side = model.SideShort
			} else {
				side = model.SideLong
			}
		}

		faTilt := clamp01(sym.CoinGlassScore*0.15) * 100
		if sym.CoinGlassScore < 0 {
			faTilt = -faTilt
		}
		strengthF := (vol*sess.VolW + cvd*sess.CvdW +
			structureScore(brokeHi, brokeLo)*sess.StructW +
			clamp01(mathAbs(vwapDev)/2.5)*sess.VwapW +
			depth*sess.DepthW + sessionComponent(now)*sess.SessW) * 100
		if best.Triggered {
			strengthF += 12
		}
		strengthF += faTilt / 10
		strength := int(strengthF)
		if strength > 100 {
			strength = 100
		}
		if strength < 25 {
			continue
		}

		willFire := in.Armed && in.CanTrade && !in.InPosition && strength >= floor
		if best.Triggered {
			willFire = willFire && GatesPass(st, sym.BTCChange5mPct, side, sess)
		}

		candidates = append(candidates, model.ProSignal{
			Symbol: sym.Symbol, Side: side, Strength: strength,
			Playbook: best.ID, Session: sess.Name,
			Price: st.LastPrice, SpreadBps: st.SpreadBps,
			QuoteVol24: st.QuoteVolume24h, IsCore: sym.IsCore,
			Components: model.ScoreComponents{
				Volume: vol, CVD: cvd, Structure: structureScore(brokeHi, brokeLo),
				Context: ctx, Depth: depth, Session: sessionComponent(now),
			},
			Extra: model.SignalExtra{
				VWAPDevPct: vwapDev, ATR: atr, EMA9: ema9, FaTilt: faTilt,
				BtcRegime: strategy.BtcRegimeTag(sym.BTCChange5mPct),
				TakerFlow: flow, CVDState: cvdState,
			},
			WillFire: willFire, UpdatedAt: now,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Strength != candidates[j].Strength {
			return candidates[i].Strength > candidates[j].Strength
		}
		return candidates[i].Symbol < candidates[j].Symbol
	})
	for i := range candidates {
		candidates[i].Rank = i + 1
	}

	btcPct := 0.0
	if len(in.Symbols) > 0 {
		btcPct = in.Symbols[0].BTCChange5mPct
	}
	regime := buildRegime(candidates, btcPct)

	return RankOutput{
		Signals: candidates,
		Floor:   floor,
		Regime:  regime,
		Session: model.SessionCockpit{
			Session: sess.Name, Floor: floor,
			TradesToday: in.TradesToday, MinTradesPerDay: in.MinTradesPerDay,
			MaxTradesPerDay: config.Live.Get().MaxTradesPerDay,
			TargetTrades:    config.TargetTradesPerDay,
			ActivePlaybooks: sess.Playbooks,
			NextFloorDrop:   NextFloorDropUTC(now, in.TradesToday, in.MinTradesPerDay),
			SignalCount:     len(candidates),
			RegimeLabel:     regime.Label,
		},
	}
}

func buildRegime(signals []model.ProSignal, btcPct float64) model.RadarRegime {
	reg := model.RadarRegime{BtcChange5mPct: btcPct, Label: "CHOP", SkipCount: 0}
	var maxS float64
	for _, s := range signals {
		sc := float64(s.Strength) / 100
		if sc > maxS {
			maxS = sc
		}
	}
	reg.TradeCount = len(signals)
	reg.MaxScore = maxS
	if len(signals) == 0 {
		reg.Summary = "no signals above floor"
		return reg
	}
	reg.Label = "SIGNALS"
	if len(signals) >= 3 {
		reg.Label = "ACTIVE"
	}
	reg.Summary = fmt.Sprintf("%d signals, max strength %d", len(signals), int(maxS*100))
	return reg
}

func sessionComponent(now time.Time) float64 {
	h := now.UTC().Hour()
	switch {
	case h >= 0 && h < 8:
		return 0.6
	case h >= 8 && h < 14:
		return 0.85
	case h >= 14 && h < 22:
		return 1.0
	default:
		return 0.75
	}
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func GatesPass(st market.SymbolState, btcPct float64, side model.Side, sess SessionProfile) bool {
	if st.SpreadBps > sess.MaxSpreadBps {
		return false
	}
	if side == model.SideLong && btcPct <= config.BTCBlockLongPct {
		return false
	}
	if side == model.SideShort && btcPct >= config.BTCBlockShortPct {
		return false
	}
	return true
}

func RiskUSDForStrength(strength int, sess SessionProfile) float64 {
	var base float64
	switch {
	case strength >= 85:
		base = 4.0
	case strength >= 75:
		base = 3.0
	case strength >= 65:
		base = 2.5
	default:
		base = 2.0
	}
	return base * sess.SizeMult
}
