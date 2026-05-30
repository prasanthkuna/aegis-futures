package aegis

import (
	"context"
	"log"

	"encore.app/alerts"
	"encore.app/binanceex"
	"encore.app/coinglass"
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
	r.SetTradingEnabled(tradingEnabled())
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

	rt.Start(ctx)
	log.Printf("aegis engine started (testnet=%v trading=%v symbols=%d)",
		useTestnet(), tradingEnabled(), len(uni.ActiveSymbols()))
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
