package config

import "time"

// Week1 defaults from PRD §10.
const (
	AccountCapitalUSD     = 1000.0
	ActiveCapitalUSD      = 250.0
	MaxLeverage           = 3
	RiskPerTradeUSD       = 1.25
	MaxOpenPositions      = 1
	MaxTradesPerDay       = 6
	MinTradesPerDay       = 2
	TargetTradesPerDay    = 4
	DailyHardStopUSD      = 7.5
	WeeklyHardStopUSD     = 20.0
	MaxConsecutiveLosses  = 3
	CooldownAfterLossMins = 20

	MinTradeScore     = 0.78
	APlusTradeScore   = 0.88
	VolumeSurgeMult   = 1.8
	SwingLookback     = 20
	BTCBlockLongPct   = -0.40
	BTCBlockShortPct  = 0.40
	CVDWindow         = 3 * time.Minute
	UniverseTopN      = 30
	UniverseRefresh   = 15 * time.Minute
	MaxNewSymbols     = 27 // 3 core + 27 rotators = 30 max
	EntryTimeout      = 5 * time.Second
	MaxEntryAttempts  = 2
	MaxTradeDuration  = 30 * time.Minute
	CoinGlassPoll     = 3 * time.Minute
	GuardianInterval  = 1 * time.Second
	EngineTick        = 2 * time.Second
)

var AlwaysInclude = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
