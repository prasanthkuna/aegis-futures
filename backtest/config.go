package backtest

// ExitParams tunes exit behavior (BT-4).
type ExitParams struct {
	BEAtR      float64
	PartialAtR float64
	PartialPct float64
	FullTPAtR  float64
	StaleHours float64
	TrailATR   float64
}

func DefaultExitParams() ExitParams {
	return ExitParams{
		BEAtR: 0.5, PartialAtR: 1.0, PartialPct: 0.4,
		FullTPAtR: 2.5, StaleHours: 72, TrailATR: 1.5,
	}
}

// RunConfig is one backtest run.
type RunConfig struct {
	Name            string
	Days            int
	UniverseTopN    int
	PlaybooksOnly   []string
	SessionsOnly    []string // empty = all sessions
	FloorOverride   int      // 0 = adaptive
	MaxTradesPerDay int      // 0 = use config.MaxTradesPerDay (6)
	Exit            ExitParams
	OOSStartFrac    float64 // 0.7 = last 30% is OOS for walk-forward style split
	SlippageMult    float64 // 1.0 = default slippage; 2.0 = stress tier
	CachedOnly      bool
}

func (c RunConfig) maxTradesDay() int {
	if c.MaxTradesPerDay > 0 {
		return c.MaxTradesPerDay
	}
	return 6
}
