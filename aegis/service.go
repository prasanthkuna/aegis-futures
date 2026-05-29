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
	cancel context.CancelFunc
}

func initService() (*Service, error) {
	hub := market.NewHub()
	net := binanceex.NetworkForTestnet(useTestnet())
	var bc *binanceex.Client
	if secrets.BinanceAPIKey != "" && secrets.BinanceAPISecret != "" {
		bc = binanceex.NewClient(secrets.BinanceAPIKey, secrets.BinanceAPISecret, useTestnet())
	}
	uni := universe.NewManager(hub, bc)
	r := risk.NewEngine()
	r.SetTradingEnabled(tradingEnabled())
	ex := execution.New(bc)
	g := guardian.New(bc, ex)
	led := ledger.New(db)
	tg := alerts.NewTelegram("", "") // optional: wire env later if needed
	cg := coinglass.NewClient("")    // context from Binance; CoinGlass not required

	rt := engine.NewRuntime(hub, uni, r, ex, g, led, tg, cg, bc)
	svc := &Service{rt: rt, hub: hub}

	ctx, cancel := context.WithCancel(context.Background())
	svc.cancel = cancel

	cb := binanceex.WSCallbacks{
		OnAggTrade:   hub.OnAggTrade,
		OnBookTicker: hub.OnBookTicker,
		OnKline:      hub.OnKline,
	}
	ws := binanceex.NewWSManager(net, cb)
	go func() {
		syms := append([]string{}, uni.ActiveSymbols()...)
		ws.Start(ctx, syms)
	}()
	go func() {
		<-ctx.Done()
		ws.Close()
	}()

	rt.Start(ctx)
	log.Printf("aegis engine started (testnet=%v trading=%v)", useTestnet(), tradingEnabled())
	return svc, nil
}

func (s *Service) Shutdown(force context.Context) {
	if s.cancel != nil {
		s.cancel()
	}
}
