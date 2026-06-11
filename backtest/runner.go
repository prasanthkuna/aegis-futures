package backtest

import (
	"context"
	"fmt"
	"time"

	"encore.app/config"
	"encore.app/execution"
	"encore.app/guardian"
	"encore.app/market"
	"encore.app/model"
	"encore.app/signal"
)

const (
	takerFeeBps   = 4.0
	slippageBps   = 2.0
	warmupBars    = 25
)

type Trade struct {
	Symbol    string
	Side      model.Side
	Playbook  string
	Session   string
	Strength  int
	EntryTime time.Time
	ExitTime  time.Time
	EntryPx   float64
	ExitPx    float64
	RMultiple float64
	PnLUSD    float64
	FeesUSD   float64
	Reason    string
	IsOOS     bool
}

type Result struct {
	Config       RunConfig
	Trades       []Trade
	NetPnL       float64
	GrossPnL     float64
	Fees         float64
	MaxDrawdown  float64
	ProfitFactor float64
	WinRate      float64
	ExpectancyR  float64
	TradeCount   int
	OOSTrades    int
	OOSNetPnL    float64
	FlatBarsPct  float64
	ByPlaybook   map[string]int
	BySession    map[string]int
}

type Runner struct {
	Store *DataStore
}

func NewRunner(store *DataStore) *Runner {
	if store == nil {
		store = NewDataStore("")
	}
	return &Runner{Store: store}
}

func (r *Runner) Run(ctx context.Context, cfg RunConfig) (*Result, error) {
	if cfg.Days <= 0 {
		cfg.Days = 60
	}
	ds, err := r.LoadDataset(ctx, cfg.Days, cfg.CachedOnly)
	if err != nil {
		return nil, err
	}
	if cfg.OOSStartFrac > 0 && cfg.OOSStartFrac < 1 {
		ds.OOSStart = ds.Timeline[int(float64(len(ds.Timeline))*cfg.OOSStartFrac)]
	}
	return r.RunDataset(ds, cfg)
}

func (r *Runner) RunDataset(ds *Dataset, cfg RunConfig) (*Result, error) {
	if ds == nil {
		return nil, fmt.Errorf("nil dataset")
	}
	if cfg.UniverseTopN <= 0 {
		cfg.UniverseTopN = 30
	}
	exitP := cfg.Exit
	if exitP.FullTPAtR == 0 {
		exitP = DefaultExitParams()
	}
	slipBps := slippageBps
	if cfg.SlippageMult > 0 {
		slipBps *= cfg.SlippageMult
	}
	maxDay := cfg.maxTradesDay()

	all := ds.All
	timeline := ds.Timeline
	idxMaps := ds.IdxMaps
	idxAt := make(map[string]int, len(all))
	for sym := range all {
		idxAt[sym] = -1
	}
	oosStart := ds.OOSStart
	hub := NewReplayHub()
	var (
		pos       *guardian.InternalPosition
		trades    []Trade
		equity    = config.ActiveCapitalUSD
		peak      = equity
		maxDD     float64
		gross     float64
		fees      float64
		flatScans int
		totalScan int
		tradesDay int
		lastDay   int
	)

	closeTrade := func(pos *guardian.InternalPosition, exitPx float64, reason string, at time.Time, isOOS bool) {
		qty := pos.RemainingQty
		if qty <= 0 {
			qty = pos.Quantity
		}
		var pnl float64
		if pos.Side == model.SideLong {
			pnl = (exitPx - pos.EntryPrice) * qty
		} else {
			pnl = (pos.EntryPrice - exitPx) * qty
		}
		fee := (pos.EntryPrice*qty + exitPx*qty) * takerFeeBps / 10000
		net := pnl - fee
		rMult := 0.0
		if pos.RiskUSD > 0 {
			rMult = pnl / pos.RiskUSD
		}
		trades = append(trades, Trade{
			Symbol: pos.Symbol, Side: pos.Side, Playbook: pos.Playbook, Session: pos.Session,
			Strength: pos.StrengthAtEntry, EntryTime: time.UnixMilli(pos.EntryTime).UTC(),
			ExitTime: at, EntryPx: pos.EntryPrice, ExitPx: exitPx,
			RMultiple: rMult, PnLUSD: net, FeesUSD: fee, Reason: reason, IsOOS: isOOS,
		})
		gross += pnl
		fees += fee
		equity += net
		if equity > peak {
			peak = equity
		}
		if dd := peak - equity; dd > maxDD {
			maxDD = dd
		}
	}

	for _, t := range timeline {
		isOOS := !t.Before(oosStart)
		day := t.YearDay()
		if day != lastDay {
			tradesDay = 0
			lastDay = day
		}

		for sym, bars := range all {
			m := idxMaps[sym]
			i, ok := m[t.UnixMilli()]
			if !ok {
				continue
			}
			idxAt[sym] = i
			qv := RollingQuoteVol24h(bars, i)
			hub.ApplyBar(sym, bars[i], qv)
			if ds.Context != nil {
				if cp, ok := LookupContext(ds.Context[sym], t); ok {
					hub.ApplyContext(sym, cp)
				}
			}
		}

		active := UniverseAt(all, idxAt, cfg.UniverseTopN, t)
		if len(active) == 0 {
			continue
		}

		if pos != nil {
			bars := all[pos.Symbol]
			idx := idxAt[pos.Symbol]
			if idx < 0 || idx >= len(bars) {
				pos = nil
				continue
			}
			b := bars[idx]
			if hit, px := stopHit(pos, b); hit {
				px *= 1 - slipBps/10000
				if pos.Side == model.SideShort {
					px *= 1 + slipBps/10000
				}
				closeTrade(pos, px, "stop", t, isOOS)
				pos = nil
				continue
			}
			st, _ := hub.Snapshot(pos.Symbol)
			atr := market.ATR(st.Candles5m, 14)
			if act, ok := evaluateExit(pos, st, b.Close, atr, pos.RiskUSD, t, exitP); ok {
				applyExitAction(pos, act)
				if act.ClosePct >= 1 || pos.RemainingQty <= 0 {
					px := b.Close
					closeTrade(pos, px, act.Reason, t, isOOS)
					pos = nil
				}
			}
			continue
		}

		totalScan++
		inputs := buildInputs(hub, active)
		sessNow := signal.CurrentSession(t).Name
		if len(cfg.SessionsOnly) > 0 && !sessionAllowed(sessNow, cfg.SessionsOnly) {
			continue
		}

		out := signal.Rank(signal.RankInput{
			Now: t, Symbols: inputs, TradesToday: tradesDay,
			MinTradesPerDay: config.MinTradesPerDay,
			Armed: true, CanTrade: tradesDay < maxDay,
			InPosition: false, TradingEnabled: true, Paused: false,
			KillSwitch: false, RiskOK: true, BotState: "SCANNING", MarketHealthy: true,
			PlaybooksOnly: cfg.PlaybooksOnly, FloorOverride: cfg.FloorOverride,
		})
		if out.Heartbeat.AboveFloor == 0 {
			flatScans++
		}

		for _, sig := range out.Signals {
			if !sig.WillFire {
				continue
			}
			st, ok := hub.Snapshot(sig.Symbol)
			if !ok || st.LastPrice <= 0 {
				continue
			}
			entry := st.LastPrice
			sess := signal.SessionAdjustments(t, signal.CurrentSession(t))
			riskUSD := signal.RiskUSDForStrength(sig.Strength, sess)
			atr := sig.Extra.ATR
			stopDist := atr * 1.5
			if stopDist <= 0 {
				stopDist = entry * 0.005
			}
			var stop float64
			if sig.Side == model.SideLong {
				stop = entry - stopDist
			} else {
				stop = entry + stopDist
			}
			qty := execution.RiskQuantity(config.ActiveCapitalUSD, riskUSD, entry, stop, config.MaxLeverage)
			if qty <= 0 {
				continue
			}
			entry *= 1 + slipBps/10000
			if sig.Side == model.SideShort {
				entry *= 1 - slipBps/10000
			}
			pos = &guardian.InternalPosition{
				Symbol: sig.Symbol, Side: sig.Side, Quantity: qty, RemainingQty: qty,
				EntryPrice: entry, StopPrice: stop, EntryTime: t.UnixMilli(),
				Playbook: sig.Playbook, StrengthAtEntry: sig.Strength, Session: sig.Session,
				ExitPhase: "PROTECTED", ATRAtEntry: atr, RiskUSD: riskUSD,
			}
			tradesDay++
			break
		}
	}

	res := &Result{
		Config: cfg, Trades: trades, NetPnL: equity - config.ActiveCapitalUSD,
		GrossPnL: gross, Fees: fees, MaxDrawdown: maxDD, TradeCount: len(trades),
		ByPlaybook: map[string]int{}, BySession: map[string]int{},
	}
	if totalScan > 0 {
		res.FlatBarsPct = float64(flatScans) / float64(totalScan) * 100
	}
	summarize(res)
	return res, nil
}

func buildInputs(hub *ReplayHub, symbols []string) []signal.SymbolInput {
	out := make([]signal.SymbolInput, 0, len(symbols))
	btc := hub.BTC5mChangePct()
	for _, sym := range symbols {
		st, ok := hub.Snapshot(sym)
		if !ok {
			continue
		}
		core := false
		for _, c := range config.AlwaysInclude {
			if c == sym {
				core = true
				break
			}
		}
		ctxScore := ContextScoreFromState(st.OIDeltaPct, st.FundingRate, st.TakerBuySellRatio)
		out = append(out, signal.SymbolInput{
			Symbol: sym, State: st, CoinGlassScore: ctxScore,
			BTCChange5mPct: btc, IsCore: core,
		})
	}
	return out
}

func summarize(res *Result) {
	var wins, losses float64
	var winCount int
	var rSum float64
	for _, tr := range res.Trades {
		res.ByPlaybook[tr.Playbook]++
		res.BySession[tr.Session]++
		if tr.IsOOS {
			res.OOSTrades++
			res.OOSNetPnL += tr.PnLUSD
		}
		rSum += tr.RMultiple
		if tr.PnLUSD > 0 {
			wins += tr.PnLUSD
			winCount++
		} else if tr.PnLUSD < 0 {
			losses += -tr.PnLUSD
		}
	}
	if res.TradeCount > 0 {
		res.WinRate = float64(winCount) / float64(res.TradeCount) * 100
		res.ExpectancyR = rSum / float64(res.TradeCount)
	}
	if losses > 0 {
		res.ProfitFactor = wins / losses
	} else if wins > 0 {
		res.ProfitFactor = 99
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func sessionAllowed(sess string, allowed []string) bool {
	for _, a := range allowed {
		if a == sess {
			return true
		}
	}
	return false
}
