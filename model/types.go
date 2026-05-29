package model

import "time"

type BotState string

const (
	StateIdle          BotState = "IDLE"
	StateScanning      BotState = "SCANNING"
	StateSetupFound    BotState = "SETUP_FOUND"
	StateRiskChecking  BotState = "RISK_CHECKING"
	StateOrderPlacing  BotState = "ORDER_PLACING"
	StateEntryPending  BotState = "ENTRY_PENDING"
	StateEntryFilled   BotState = "ENTRY_FILLED"
	StateStopPlacing   BotState = "STOP_PLACING"
	StateInPosition    BotState = "IN_POSITION"
	StateExitPending   BotState = "EXIT_PENDING"
	StateClosed        BotState = "CLOSED"
	StateCooldown      BotState = "COOLDOWN"
	StatePaused        BotState = "PAUSED"
	StateError         BotState = "ERROR"
	StateKillSwitch    BotState = "KILL_SWITCH"
)

type Side string

const (
	SideLong  Side = "LONG"
	SideShort Side = "SHORT"
)

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
}

type OpenPositionView struct {
	Symbol         string  `json:"symbol"`
	Side           Side    `json:"side"`
	EntryPrice     float64 `json:"entryPrice"`
	CurrentPrice   float64 `json:"currentPrice"`
	Quantity       float64 `json:"quantity"`
	Leverage       int     `json:"leverage"`
	StopPrice      float64 `json:"stopPrice"`
	TakeProfitPrice float64 `json:"takeProfitPrice"`
	UnrealizedPnL  float64 `json:"unrealizedPnl"`
	FeesSoFar      float64 `json:"feesSoFar"`
	RMultiple      float64 `json:"rMultiple"`
	TimeInTradeSec int64   `json:"timeInTradeSec"`
	GuardianStatus string  `json:"guardianStatus"`
}

type DashboardSummary struct {
	Mode              string  `json:"mode"`
	BotStatus         string  `json:"botStatus"`
	ActiveCapitalUsd  float64 `json:"activeCapitalUsd"`
	AccountBalance    float64 `json:"accountBalance"`
	AvailableMargin   float64 `json:"availableMargin"`
	OpenPnL           float64 `json:"openPnl"`
	RealizedPnL       float64 `json:"realizedPnl"`
	NetPnLAfterFees   float64 `json:"netPnlAfterFees"`
	TodayPnL          float64 `json:"todayPnl"`
	WeeklyPnL         float64 `json:"weeklyPnl"`
	FeesPaid          float64 `json:"feesPaid"`
	FundingPaid       float64 `json:"fundingPaid"`
	CurrentDrawdown   float64 `json:"currentDrawdown"`
	KillSwitchActive  bool    `json:"killSwitchActive"`
	LastTradeSymbol   string  `json:"lastTradeSymbol"`
	HasOpenPosition   bool    `json:"hasOpenPosition"`
	TradingEnabled    bool    `json:"tradingEnabled"`
	Testnet           bool    `json:"testnet"`
}
