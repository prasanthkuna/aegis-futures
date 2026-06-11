package config

// BTC 1h swing prod defaults from backtest (liberal book, 60d BTCUSDT).
const (
	BTCProdRiskUSD         = 5.0
	BTCProdMaxLeverage     = 5
	BTCProdMaxTradesPerDay = 4 // grid best for net; 6/day overtrades in combo book
	BTCProdActiveCapital   = 250.0
	BTCProdTimeframeBars   = 12 // 12 × 5m = 1h
	BTCProdRR              = 4.0
	BTCProdStopATR         = 1.4
	BTCProdPlaybook        = "S4_SQUEEZE_LIBERAL"
)

// BTCProdSnapshot returns risk settings for BTC swing mode.
func BTCProdSnapshot() Snapshot {
	s := Live.Get()
	s.RiskPerTradeUSD = BTCProdRiskUSD
	s.MaxLeverage = BTCProdMaxLeverage
	s.MaxTradesPerDay = BTCProdMaxTradesPerDay
	s.ActiveCapitalUSD = BTCProdActiveCapital
	s.MaxOpenPositions = 1
	return s
}
