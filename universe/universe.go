package universe

import (
	"context"
	"sort"
	"strings"

	"encore.app/binanceex"
	"encore.app/config"
	"encore.app/market"
)

type Entry struct {
	Symbol         string
	Rank           int
	QuoteVolume24h float64
	SpreadBps      float64
	Tradable       bool
	Reason         string
}

type Manager struct {
	hub    *market.Hub
	client *binanceex.Client
	active []string
}

func NewManager(hub *market.Hub, client *binanceex.Client) *Manager {
	return &Manager{hub: hub, client: client, active: append([]string{}, config.AlwaysInclude...)}
}

func (m *Manager) ActiveSymbols() []string {
	out := make([]string, len(m.active))
	copy(out, m.active)
	return out
}

func (m *Manager) Refresh(ctx context.Context) ([]Entry, error) {
	tickers, err := m.client.Ticker24hrAll(ctx)
	if err != nil {
		return nil, err
	}
	type row struct {
		symbol string
		vol    float64
	}
	var rows []row
	for _, t := range tickers {
		if !strings.HasSuffix(t.Symbol, "USDT") {
			continue
		}
		vol := binanceex.ParseFloat(t.QuoteVolume)
		if vol <= 0 {
			continue
		}
		rows = append(rows, row{symbol: t.Symbol, vol: vol})
		m.hub.SetQuoteVolume(t.Symbol, vol)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].vol > rows[j].vol })

	seen := map[string]bool{}
	var next []string
	for _, s := range config.AlwaysInclude {
		seen[s] = true
		next = append(next, s)
	}
	newCount := 0
	for _, r := range rows {
		if len(next) >= config.UniverseTopN {
			break
		}
		if seen[r.symbol] {
			continue
		}
		if newCount >= config.MaxNewSymbols {
			continue
		}
		seen[r.symbol] = true
		next = append(next, r.symbol)
		newCount++
	}
	m.active = next

	var entries []Entry
	for i, sym := range m.active {
		st, ok := m.hub.Snapshot(sym)
		spread := 999.0
		if ok {
			spread = st.SpreadBps
		}
		tradable := spread < 15
		reason := "ok"
		if !tradable {
			reason = "spread_too_wide"
		}
		vol := 0.0
		if ok {
			vol = st.QuoteVolume24h
		}
		entries = append(entries, Entry{
			Symbol: sym, Rank: i + 1, QuoteVolume24h: vol,
			SpreadBps: spread, Tradable: tradable, Reason: reason,
		})
	}
	return entries, nil
}
