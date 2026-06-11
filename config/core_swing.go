package config

// Core swing mode — validated BTC/ETH/SOL 1h strategies (90d+180d).

const (
	CoreSwing5mKeep      = 800 // ~66h of 5m bars for 1h indicators
	CoreSwing1hBars      = 12  // 12 × 5m = 1h
	CoreSwingMaxHoldHrs  = 36
	CoreSwingRiskPct     = 2.0 // % of active capital per trade
)

// TradingMode values for Snapshot.TradingMode.
const (
	TradingModeAltScan   = "alt_scan"
	TradingModeCoreSwing = "core_swing"
)

// CoreSwingSpec is the fixed prod playbook per symbol.
type CoreSwingSpec struct {
	Symbol     string
	Playbook   string
	StopATR    float64
	TargetRR   float64
	EvalLiberal bool // S4/S11 variant flags handled in signal package
}

var coreSwingSpecs = map[string]CoreSwingSpec{
	"BTCUSDT": {Symbol: "BTCUSDT", Playbook: "S4_SQUEEZE_LIBERAL", StopATR: 1.4, TargetRR: 4.0},
	"SOLUSDT": {Symbol: "SOLUSDT", Playbook: "S4_SQUEEZE_LIBERAL", StopATR: 1.8, TargetRR: 4.0},
	"ETHUSDT": {Symbol: "ETHUSDT", Playbook: "S11_EMA_TREND_STRICT", StopATR: 1.8, TargetRR: 4.0},
}

func CoreSwingSpecFor(symbol string) (CoreSwingSpec, bool) {
	s, ok := coreSwingSpecs[symbol]
	return s, ok
}

func CoreSwingSymbols() []string {
	return append([]string(nil), AlwaysInclude...)
}

var (
	coreSwingEnabled   bool
	coreSwingAggressive bool
)

// SetCoreSwingMode sets runtime flags (from secrets at startup).
func SetCoreSwingMode(enabled, aggressive bool) {
	coreSwingEnabled = enabled
	coreSwingAggressive = aggressive
}

func IsCoreSwingMode() bool { return coreSwingEnabled }
func IsCoreSwingAggressive() bool { return coreSwingAggressive }

// CoreSwingConservativeSnapshot — start here: $200, $4 risk, 5x, max 4/day.
func CoreSwingConservativeSnapshot() Snapshot {
	return Snapshot{
		TradingMode:       TradingModeCoreSwing,
		ActiveCapitalUSD:  200,
		RiskPerTradeUSD:   4.0,
		MaxLeverage:       5,
		MaxOpenPositions:  1,
		MaxTradesPerDay:   4,
		MinTradesPerDay:   0,
		TargetTradesPerDay: 2,
		DailyHardStopUSD:  12,
		WeeklyHardStopUSD: 30,
		MinTradeScore:     0,
	}
}

// CoreSwingAggressiveSnapshot — enable after live validation (AegisAggressiveMode=1).
func CoreSwingAggressiveSnapshot() Snapshot {
	return Snapshot{
		TradingMode:       TradingModeCoreSwing,
		ActiveCapitalUSD:  250,
		RiskPerTradeUSD:   5.0,
		MaxLeverage:       5,
		MaxOpenPositions:  1,
		MaxTradesPerDay:   4,
		MinTradesPerDay:   0,
		TargetTradesPerDay: 2,
		DailyHardStopUSD:  15,
		WeeklyHardStopUSD: 40,
		MinTradeScore:     0,
	}
}

func ApplyCoreSwingLiveConfig() {
	if coreSwingAggressive {
		Live.Apply(CoreSwingAggressiveSnapshot())
	} else {
		Live.Apply(CoreSwingConservativeSnapshot())
	}
}
