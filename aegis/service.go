package aegis

// deploy-smoke: GitHub → Encore auto-deploy test (2026-06-01)

import (
	"context"
	"log"

	"encore.app/alerts"
	"encore.app/binanceex"
	"encore.app/coinglass"
	"encore.app/config"
	"encore.app/engine"
	"encore.app/execution"
	"encore.app/guardian"
	"encore.app/ledger"
	"encore.app/market"
	"encore.app/risk"
	"encore.app/universe"
)

//encore:service
type Service struct {
	rt     *engine.Runtime
	hub    *market.Hub
	ws     *binanceex.WSManager
	cancel context.CancelFunc
	radar  radarCache
}

func initService() (*Service, error) {
	hub := market.NewHub()
	net := binanceex.NetworkForTestnet(useTestnet())
	var bc *binanceex.Client
	if hasBinanceKeys() {
		bc = binanceex.NewClient(binanceAPIKey(), binanceAPISecret(), useTestnet())
	}
	uni := universe.NewManager(hub, bc)
	r := risk.NewEngine()
	config.SetPaperMode(paperModeEnabled())
	r.SetTradingEnabled(tradingEnabled() || paperModeEnabled())
	ex := execution.New(bc)
	g := guardian.New(bc, ex)
	led := ledger.New(db)
	tg := alerts.NewTelegram("", "")
	cg := coinglass.NewClient("")

	rt := engine.NewRuntime(hub, uni, r, ex, g, led, tg, cg, bc)
	svc := &Service{rt: rt, hub: hub}

	ctx, cancel := context.WithCancel(context.Background())
	svc.cancel = cancel

	if err := loadBotConfig(ctx); err != nil {
		log.Printf("bot_config load (defaults): %v", err)
	}

	config.SetCoreSwingMode(coreSwingEnabled(), aggressiveMode())
	if config.IsCoreSwingMode() {
		config.ApplyCoreSwingLiveConfig()
		uni.SetCoreOnly(true)
		log.Printf("core swing mode ON (conservative=%v)", !config.IsCoreSwingAggressive())
	}

	cb := binanceex.WSCallbacks{
		OnAggTrade:   hub.OnAggTrade,
		OnBookTicker: hub.OnBookTicker,
		OnKline:      hub.OnKline,
	}
	ws := binanceex.NewWSManager(net, cb)
	svc.ws = ws

	rt.OnUniverseChanged = func(symbols []string) {
		ws.ReplaceStreams(ctx, symbols)
	}

	// Initial universe + WS before engine loops.
	if bc != nil {
		if _, err := uni.Refresh(ctx); err != nil {
			log.Printf("initial universe refresh: %v", err)
		}
	}
	ws.ReplaceStreams(ctx, uni.ActiveSymbols())

	if config.IsCoreSwingMode() && bc != nil {
		for _, sym := range config.CoreSwingSymbols() {
			bars, err := bc.FetchKlines5mHistory(ctx, sym, config.CoreSwing5mKeep)
			if err != nil {
				log.Printf("seed klines %s: %v", sym, err)
				continue
			}
			hub.SeedCandles5m(sym, bars)
			log.Printf("seeded %s: %d 5m bars", sym, len(bars))
		}
	}

	rt.Start(ctx)
	mode := "alt_scan"
	if config.IsCoreSwingMode() {
		mode = "core_swing"
	}
	log.Printf("aegis engine started mode=%s testnet=%v trading=%v paper=%v symbols=%d",
		mode, useTestnet(), tradingEnabled(), paperModeEnabled(), len(uni.ActiveSymbols()))
	return svc, nil
}

func (s *Service) Shutdown(force context.Context) {
	if s.cancel != nil {
		s.cancel()
	}
	if s.ws != nil {
		s.ws.Close()
	}
}
