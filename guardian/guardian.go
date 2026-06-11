package guardian

import (
	"context"
	"fmt"
	"math"

	"encore.app/binanceex"
	"encore.app/execution"
	"encore.app/model"
)

type CheckResult struct {
	OK      bool
	Details string
	Action  string
}

type Service struct {
	Client    *binanceex.Client
	Execution *execution.Service
}

func New(client *binanceex.Client, exec *execution.Service) *Service {
	return &Service{Client: client, Execution: exec}
}

type InternalPosition struct {
	ID              int64
	Symbol          string
	Side            model.Side
	Quantity        float64
	RemainingQty    float64
	EntryPrice      float64
	StopPrice       float64
	TakeProfitPrice float64
	EntryTime       int64 // unix ms
	HasStop         bool
	StopOrderID     int64
	Playbook        string
	StrengthAtEntry int
	Session         string
	ExitPhase       string
	PeakR           float64
	PartialPct      float64
	ATRAtEntry      float64
	RiskUSD         float64
	TargetRR        float64
	MaxHoldHours    float64
	Paper           bool
}

func (g *Service) Verify(ctx context.Context, pos *InternalPosition) CheckResult {
	if pos == nil {
		return CheckResult{OK: true, Details: "flat"}
	}
	if pos.Paper {
		return CheckResult{OK: true, Details: "paper_simulated"}
	}
	if !pos.HasStop || pos.StopPrice <= 0 {
		return CheckResult{OK: false, Details: "missing_stop", Action: "emergency_close"}
	}
	if g.Client == nil {
		return CheckResult{OK: false, Details: "no_binance_client", Action: "pause"}
	}
	risks, err := g.Client.PositionRisk(ctx, pos.Symbol)
	if err != nil {
		return CheckResult{OK: false, Details: fmt.Sprintf("position_risk_err: %v", err), Action: "pause"}
	}
	var exchQty float64
	for _, r := range risks {
		if r.Symbol == pos.Symbol {
			exchQty = binanceex.ParseFloat(r.PositionAmt)
			break
		}
	}
	want := pos.Quantity
	if pos.Side == model.SideShort {
		want = -pos.Quantity
	}
	if math.Abs(exchQty-want) > 1e-6 {
		return CheckResult{OK: false, Details: fmt.Sprintf("qty_mismatch internal=%.6f exchange=%.6f", want, exchQty), Action: "pause"}
	}
	return CheckResult{OK: true, Details: "ok"}
}

func (g *Service) HandleFailure(ctx context.Context, pos *InternalPosition, res CheckResult) error {
	if res.Action == "emergency_close" && pos != nil && g.Execution != nil {
		return g.Execution.EmergencyClose(ctx, pos.Symbol, pos.Side, pos.Quantity)
	}
	return nil
}
