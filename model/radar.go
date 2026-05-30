package model

import "time"

type ScoreComponents struct {
	Volume    float64 `json:"volume"`
	CVD       float64 `json:"cvd"`
	Structure float64 `json:"structure"`
	Context   float64 `json:"context"`
	Depth     float64 `json:"depth"`
	Session   float64 `json:"session"`
}

type GateFlags struct {
	MinScore  bool `json:"minScore"`
	Structure bool `json:"structure"`
	Flow      bool `json:"flow"`
	Btc       bool `json:"btc"`
	Spread    bool `json:"spread"`
}

type SymbolSnapshot struct {
	Symbol           string    `json:"symbol"`
	Rank             int       `json:"rank"`
	QuoteVolume24h   float64   `json:"quoteVolume24h"`
	Price            float64   `json:"price"`
	SpreadBps        float64   `json:"spreadBps"`
	VolumeSurge      float64   `json:"volumeSurge"`
	CVDState         string    `json:"cvdState"`
	TakerFlow        string    `json:"takerFlow"`
	OIFundingContext string    `json:"oiFundingContext"`
	CoinGlassScore   float64   `json:"coinGlassScore"`
	SessionScore     float64   `json:"sessionScore"`
	TradeScore       float64   `json:"tradeScore"`
	Decision         string    `json:"decision"`
	Reason           string    `json:"reason"`
	UpdatedAt        time.Time `json:"updatedAt"`

	GapToTrade     float64         `json:"gapToTrade"`
	WeakestLink    string          `json:"weakestLink"`
	Tier           string          `json:"tier"`
	SideHint       string          `json:"sideHint"`
	Components     ScoreComponents `json:"components"`
	Gates          GateFlags       `json:"gates"`
	BtcRegime      string          `json:"btcRegime"`
	IsCore         bool            `json:"isCore"`
	ScoreDelta     float64         `json:"scoreDelta"`
	PriceDeltaPct  float64         `json:"priceDeltaPct"`
	SurgeDelta     float64         `json:"surgeDelta"`
	WillFire       bool            `json:"willFire"`
	HasOpenSlot    bool            `json:"hasOpenSlot"`
}

type RadarMeta struct {
	MinTradeScore     float64 `json:"minTradeScore"`
	WatchMinScore     float64 `json:"watchMinScore"`
	APlusTradeScore   float64 `json:"aPlusTradeScore"`
	Armed             bool    `json:"armed"`
	TradingEnabled    bool    `json:"tradingEnabled"`
	Paused            bool    `json:"paused"`
	KillSwitch        bool    `json:"killSwitch"`
	TradesToday       int     `json:"tradesToday"`
	MaxTradesPerDay   int     `json:"maxTradesPerDay"`
	OpenPositions     int     `json:"openPositions"`
	MaxOpenPositions  int     `json:"maxOpenPositions"`
	TodayPnL          float64 `json:"todayPnl"`
	DailyHardStopUsd  float64 `json:"dailyHardStopUsd"`
}

type RadarRegime struct {
	Label          string  `json:"label"`
	Summary        string  `json:"summary"`
	TradeCount     int     `json:"tradeCount"`
	WatchCount     int     `json:"watchCount"`
	SkipCount      int     `json:"skipCount"`
	MaxScore       float64 `json:"maxScore"`
	MedianSurge    float64 `json:"medianSurge"`
	BtcChange5mPct float64 `json:"btcChange5mPct"`
}
