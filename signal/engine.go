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
	Now             time.Time
	Symbols         []SymbolInput
	TradesToday     int
	MinTradesPerDay int
	Armed           bool
	CanTrade        bool
	InPosition      bool
	TradingEnabled  bool
	Paused          bool
	KillSwitch      bool
	RiskOK          bool
	BotState        string
	MarketHealthy   bool
}

type RankOutput struct {
	Universe  []model.ProSignal
	Signals   []model.ProSignal
	NearMiss  []model.ProSignal
	Session   model.SessionCockpit
	Floor     int
	Regime    model.RadarRegime
	Heartbeat model.EngineHeartbeat
	Narrative string
}

func Rank(in RankInput) RankOutput {
	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sess := SessionAdjustments(now, CurrentSession(now))
	floor := AdaptiveFloor(sess, now, in.TradesToday, in.MinTradesPerDay)
	chop := true

	var universe []model.ProSignal
	flatCVD := 0

	for _, sym := range in.Symbols {
		sig, flat := evalSymbol(sym, sess, floor, now, chop, in)
		if flat {
			flatCVD++
		}
		universe = append(universe, sig)
	}

	sort.Slice(universe, func(i, j int) bool {
		if universe[i].Strength != universe[j].Strength {
			return universe[i].Strength > universe[j].Strength
		}
		if universe[i].QuoteVol24 != universe[j].QuoteVol24 {
			return universe[i].QuoteVol24 > universe[j].QuoteVol24
		}
		return universe[i].Symbol < universe[j].Symbol
	})
	for i := range universe {
		universe[i].Rank = i + 1
	}

	var signals, nearMiss []model.ProSignal
	willFire := 0
	for _, s := range universe {
		if s.Strength >= floor {
			signals = append(signals, s)
		}
		if s.Strength >= floor-10 && s.Strength < floor {
			nearMiss = append(nearMiss, s)
		}
		if s.WillFire {
			willFire++
		}
	}

	btcPct := 0.0
	if len(in.Symbols) > 0 {
		btcPct = in.Symbols[0].BTCChange5mPct
	}
	regime := buildRegime(universe, signals, floor, btcPct)
	narrative := BuildNarrative(universe, floor, flatCVD, len(in.Symbols))

	hb := model.EngineHeartbeat{
		LastScanAt:        now,
		SymbolsScanned:    len(in.Symbols),
		Candidates:        len(universe),
		AboveFloor:        len(signals),
		NearMissCount:     len(nearMiss),
		WillFireCount:     willFire,
		MaxStrength:       0,
		MedianStrength:    MedianStrength(universe),
		FlatCVDCount:      flatCVD,
		MarketDataHealthy: in.MarketHealthy,
		BotState:          in.BotState,
		UniverseSize:      len(in.Symbols),
	}
	if len(universe) > 0 {
		hb.MaxStrength = universe[0].Strength
	}

	return RankOutput{
		Universe: universe,
		Signals:  signals,
		NearMiss: nearMiss,
		Floor:    floor,
		Regime:   regime,
		Narrative: narrative,
		Heartbeat: hb,
		Session: model.SessionCockpit{
			Session: sess.Name, Floor: floor,
			TradesToday: in.TradesToday, MinTradesPerDay: in.MinTradesPerDay,
			MaxTradesPerDay: config.Live.Get().MaxTradesPerDay,
			TargetTrades:    config.TargetTradesPerDay,
			ActivePlaybooks: sess.Playbooks,
			NextFloorDrop:   NextFloorDropUTC(now, in.TradesToday, in.MinTradesPerDay),
			SignalCount:     len(signals),
			RegimeLabel:     regime.Label,
		},
	}
}

func evalSymbol(sym SymbolInput, sess SessionProfile, floor int, now time.Time, chop bool, in RankInput) (model.ProSignal, bool) {
	st := sym.State
	vol := volumeSurge(st.Candles5m)
	cvd, flow, cvdState := cvdMetrics(st)
	flatCVD := cvdState == "flat"
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
	best := PlaybookResult{ID: "SCANNING", Score: 0.05}
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

	side := best.Side
	if side == "" {
		switch {
		case brokeHi:
			side = model.SideLong
		case brokeLo:
			side = model.SideShort
		case flow == "buy":
			side = model.SideLong
		case flow == "sell":
			side = model.SideShort
		default:
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
	if strength < 1 {
		strength = 1
	}
	if strength > 100 {
		strength = 100
	}

	comps := model.ScoreComponents{
		Volume: vol, CVD: cvd, Structure: structureScore(brokeHi, brokeLo),
		Context: ctx, Depth: depth, Session: sessionComponent(now),
	}
	gatesOK := true
	if best.Triggered || strength >= floor-5 {
		gatesOK = GatesPass(st, sym.BTCChange5mPct, side, sess)
	}

	willFire := in.Armed && in.CanTrade && !in.InPosition && strength >= floor && gatesOK
	canExec, block := blockReason(in, strength, floor, gatesOK)
	gap := floor - strength
	if gap < 0 {
		gap = 0
	}

	return model.ProSignal{
		Symbol: sym.Symbol, Side: side, Strength: strength,
		Playbook: best.ID, Session: sess.Name,
		Price: st.LastPrice, SpreadBps: st.SpreadBps,
		QuoteVol24: st.QuoteVolume24h, IsCore: sym.IsCore,
		Components: comps,
		Extra: model.SignalExtra{
			VWAPDevPct: vwapDev, ATR: atr, EMA9: ema9, FaTilt: faTilt,
			BtcRegime: strategy.BtcRegimeTag(sym.BTCChange5mPct),
			TakerFlow: flow, CVDState: cvdState,
		},
		WillFire: willFire, UpdatedAt: now,
		Tier: TierFor(strength, floor), GapToFloor: gap,
		CanExecute: canExec, BlockReason: block,
		PlaybookTriggered: best.Triggered,
		WeakestLink:       WeakestLink(comps),
	}, flatCVD
}

func buildRegime(universe, above []model.ProSignal, floor int, btcPct float64) model.RadarRegime {
	reg := model.RadarRegime{BtcChange5mPct: btcPct, Label: "CHOP", SkipCount: 0}
	var maxS float64
	for _, s := range universe {
		sc := float64(s.Strength) / 100
		if sc > maxS {
			maxS = sc
		}
	}
	reg.TradeCount = len(above)
	reg.WatchCount = 0
	reg.MaxScore = maxS
	if len(universe) == 0 {
		reg.Summary = "awaiting market data"
		return reg
	}
	if len(above) == 0 {
		reg.Summary = fmt.Sprintf("0 above floor %d · scanning %d symbols", floor, len(universe))
		return reg
	}
	reg.Label = "SIGNALS"
	if len(above) >= 3 {
		reg.Label = "ACTIVE"
	}
	reg.Summary = fmt.Sprintf("%d above floor · max strength %d", len(above), int(maxS*100))
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
