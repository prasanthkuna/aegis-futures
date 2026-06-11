package engine

import (
	"sync"
	"time"

	"encore.app/config"
	"encore.app/guardian"
	"encore.app/model"
)

type PaperTrade struct {
	ID         int64
	Symbol     string
	Side       model.Side
	EntryTime  time.Time
	ExitTime   time.Time
	EntryPrice float64
	ExitPrice  float64
	Quantity   float64
	GrossPnL   float64
	Fees       float64
	NetPnL     float64
	RMultiple  float64
	ExitReason string
	Playbook   string
	Session    string
}

type PaperLedger struct {
	mu     sync.RWMutex
	trades []PaperTrade
	nextID int64
}

func NewPaperLedger() *PaperLedger {
	return &PaperLedger{}
}

func paperFee(entry, exit, qty float64) float64 {
	return (entry*qty + exit*qty) * config.PaperTakerFeeBps / 10000
}

func paperGrossPnL(side model.Side, entry, exit, qty float64) float64 {
	if side == model.SideLong {
		return (exit - entry) * qty
	}
	return (entry - exit) * qty
}

func paperExitPrice(pos *guardian.InternalPosition, mark float64, reason string) float64 {
	switch reason {
	case "core_stop":
		return pos.StopPrice
	case "core_tp_rr":
		if pos.TakeProfitPrice > 0 {
			return pos.TakeProfitPrice
		}
	}
	if mark > 0 {
		return mark
	}
	return pos.EntryPrice
}

func (l *PaperLedger) Close(pos *guardian.InternalPosition, mark float64, reason string) PaperTrade {
	exitPx := paperExitPrice(pos, mark, reason)
	qty := pos.Quantity
	if pos.RemainingQty > 0 {
		qty = pos.RemainingQty
	}
	gross := paperGrossPnL(pos.Side, pos.EntryPrice, exitPx, qty)
	fees := paperFee(pos.EntryPrice, exitPx, qty)
	net := gross - fees
	risk := pos.RiskUSD
	if risk <= 0 {
		risk = config.Live.Get().RiskPerTradeUSD
	}
	rMult := 0.0
	if risk > 0 {
		rMult = net / risk
	}
	now := time.Now().UTC()
	t := PaperTrade{
		Symbol: pos.Symbol, Side: pos.Side,
		EntryTime: time.UnixMilli(pos.EntryTime).UTC(), ExitTime: now,
		EntryPrice: pos.EntryPrice, ExitPrice: exitPx, Quantity: qty,
		GrossPnL: gross, Fees: fees, NetPnL: net, RMultiple: rMult,
		ExitReason: reason, Playbook: pos.Playbook, Session: pos.Session,
	}
	l.mu.Lock()
	l.nextID++
	t.ID = l.nextID
	l.trades = append([]PaperTrade{t}, l.trades...)
	l.mu.Unlock()
	return t
}

func (l *PaperLedger) ClosedTrades() []PaperTrade {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]PaperTrade, len(l.trades))
	copy(out, l.trades)
	return out
}

func (l *PaperLedger) RealizedSummary(now time.Time) (today, week, fees float64) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	weekStart := dayStart.AddDate(0, 0, -int(now.Weekday()))
	for _, t := range l.trades {
		fees += t.Fees
		if t.ExitTime.After(dayStart) || t.ExitTime.Equal(dayStart) {
			today += t.NetPnL
		}
		if t.ExitTime.After(weekStart) || t.ExitTime.Equal(weekStart) {
			week += t.NetPnL
		}
	}
	return today, week, fees
}

func (l *PaperLedger) StrategyTruth() (wins, total int, avgWin, avgLoss, sumWin, sumLossAbs, expectancy, maxDD float64) {
	l.mu.RLock()
	trades := append([]PaperTrade(nil), l.trades...)
	l.mu.RUnlock()
	if len(trades) == 0 {
		return
	}
	total = len(trades)
	var peak, cum float64
	for i := len(trades) - 1; i >= 0; i-- {
		t := trades[i]
		cum += t.NetPnL
		if cum > peak {
			peak = cum
		}
		if dd := peak - cum; dd > maxDD {
			maxDD = dd
		}
		expectancy += t.NetPnL
		if t.NetPnL >= 0 {
			wins++
			sumWin += t.NetPnL
		} else {
			sumLossAbs += -t.NetPnL
		}
	}
	expectancy /= float64(total)
	if wins > 0 {
		avgWin = sumWin / float64(wins)
	}
	losses := total - wins
	if losses > 0 {
		avgLoss = sumLossAbs / float64(losses)
	}
	return
}

func (l *PaperLedger) MaxDrawdown() float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var peak, maxDD, cum float64
	for i := len(l.trades) - 1; i >= 0; i-- {
		cum += l.trades[i].NetPnL
		if cum > peak {
			peak = cum
		}
		if dd := peak - cum; dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}
