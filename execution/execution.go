package execution

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"time"

	"encore.app/binanceex"
	"encore.app/config"
	"encore.app/model"
)

type Service struct {
	Client *binanceex.Client
}

func New(client *binanceex.Client) *Service {
	return &Service{Client: client}
}

type EntryRequest struct {
	Symbol     string
	Side       model.Side
	Quantity   float64
	LimitPrice float64
}

type EntryResult struct {
	OrderID    int64
	FilledQty  float64
	FilledPx   float64
	Status     string
}

func (s *Service) PlacePostOnlyEntry(ctx context.Context, req EntryRequest) (*EntryResult, error) {
	if s.Client == nil {
		return nil, fmt.Errorf("binance client not configured")
	}
	_ = s.Client.SetLeverage(ctx, req.Symbol, config.MaxLeverage)
	side := "BUY"
	if req.Side == model.SideShort {
		side = "SELL"
	}
	p := url.Values{}
	p.Set("symbol", req.Symbol)
	p.Set("side", side)
	p.Set("type", "LIMIT")
	p.Set("timeInForce", "GTX")
	p.Set("quantity", fmt.Sprintf("%.6f", req.Quantity))
	p.Set("price", fmt.Sprintf("%.2f", req.LimitPrice))
	p.Set("newClientOrderId", fmt.Sprintf("aegis_e_%d", time.Now().UnixNano()))

	var lastErr error
	for attempt := 0; attempt < config.MaxEntryAttempts; attempt++ {
		ctxAttempt, cancel := context.WithTimeout(ctx, config.EntryTimeout)
		resp, err := s.Client.PlaceOrder(ctxAttempt, p)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		qty := binanceex.ParseFloat(resp.ExecutedQty)
		px := binanceex.ParseFloat(resp.AvgPrice)
		if qty > 0 {
			return &EntryResult{OrderID: resp.OrderID, FilledQty: qty, FilledPx: px, Status: resp.Status}, nil
		}
		_ = s.Client.CancelOrder(ctx, req.Symbol, resp.OrderID)
		lastErr = fmt.Errorf("entry not filled: %s", resp.Status)
	}
	return nil, lastErr
}

func (s *Service) PlaceStopMarket(ctx context.Context, symbol string, side model.Side, qty, stopPrice float64) (int64, error) {
	closeSide := "SELL"
	if side == model.SideShort {
		closeSide = "BUY"
	}
	p := url.Values{}
	p.Set("symbol", symbol)
	p.Set("side", closeSide)
	p.Set("type", "STOP_MARKET")
	p.Set("stopPrice", fmt.Sprintf("%.2f", stopPrice))
	p.Set("closePosition", "false")
	p.Set("quantity", fmt.Sprintf("%.6f", qty))
	p.Set("reduceOnly", "true")
	p.Set("workingType", "MARK_PRICE")
	p.Set("newClientOrderId", fmt.Sprintf("aegis_sl_%d", time.Now().UnixNano()))
	resp, err := s.Client.PlaceOrder(ctx, p)
	if err != nil {
		return 0, err
	}
	return resp.OrderID, nil
}

func (s *Service) PlaceTakeProfit(ctx context.Context, symbol string, side model.Side, qty, tpPrice float64) (int64, error) {
	closeSide := "SELL"
	if side == model.SideShort {
		closeSide = "BUY"
	}
	p := url.Values{}
	p.Set("symbol", symbol)
	p.Set("side", closeSide)
	p.Set("type", "TAKE_PROFIT_MARKET")
	p.Set("stopPrice", fmt.Sprintf("%.2f", tpPrice))
	p.Set("quantity", fmt.Sprintf("%.6f", qty))
	p.Set("reduceOnly", "true")
	p.Set("workingType", "MARK_PRICE")
	p.Set("newClientOrderId", fmt.Sprintf("aegis_tp_%d", time.Now().UnixNano()))
	resp, err := s.Client.PlaceOrder(ctx, p)
	if err != nil {
		return 0, err
	}
	return resp.OrderID, nil
}

func (s *Service) EmergencyClose(ctx context.Context, symbol string, side model.Side, qty float64) error {
	closeSide := "SELL"
	if side == model.SideShort {
		closeSide = "BUY"
	}
	p := url.Values{}
	p.Set("symbol", symbol)
	p.Set("side", closeSide)
	p.Set("type", "MARKET")
	p.Set("quantity", fmt.Sprintf("%.6f", qty))
	p.Set("reduceOnly", "true")
	_, err := s.Client.PlaceOrder(ctx, p)
	return err
}

func RiskQuantity(activeCapital, riskUSD, entry, stop float64, leverage int) float64 {
	stopDist := math.Abs(entry - stop)
	if stopDist <= 0 {
		return 0
	}
	notional := riskUSD / (stopDist / entry)
	maxNotional := activeCapital * float64(leverage)
	if notional > maxNotional {
		notional = maxNotional
	}
	return notional / entry
}

func StopPrice(entry float64, side model.Side, riskUSD, qty float64) float64 {
	if qty <= 0 {
		return 0
	}
	dist := riskUSD / qty
	if side == model.SideLong {
		return entry - dist
	}
	return entry + dist
}

func TakeProfitPrice(entry float64, side model.Side, rMultiple float64, riskUSD, qty float64) float64 {
	dist := riskUSD / qty * rMultiple
	if side == model.SideLong {
		return entry + dist
	}
	return entry - dist
}

func ParseBoolSecret(v string) bool {
	b, _ := strconv.ParseBool(v)
	return b
}
