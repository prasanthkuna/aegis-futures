package model



import "time"



type ProSignal struct {

	Symbol            string          `json:"symbol"`

	Rank              int             `json:"rank"`

	Side              Side            `json:"side"`

	Strength          int             `json:"strength"`

	Playbook          string          `json:"playbook"`

	Session           string          `json:"session"`

	Price             float64         `json:"price"`

	SpreadBps         float64         `json:"spreadBps"`

	Components        ScoreComponents `json:"components"`

	Extra             SignalExtra     `json:"extra"`

	WillFire          bool            `json:"willFire"`

	QuoteVol24        float64         `json:"quoteVolume24h"`

	IsCore            bool            `json:"isCore"`

	UpdatedAt         time.Time       `json:"updatedAt"`

	Tier              string          `json:"tier"`

	GapToFloor        int             `json:"gapToFloor"`

	CanExecute        bool            `json:"canExecute"`

	BlockReason       string          `json:"blockReason,omitempty"`

	PlaybookTriggered bool            `json:"playbookTriggered"`

	WeakestLink       string          `json:"weakestLink"`

}



type SignalExtra struct {

	VWAPDevPct float64 `json:"vwapDevPct"`

	ATR        float64 `json:"atr"`

	EMA9       float64 `json:"ema9"`

	FaTilt     float64 `json:"faTilt"`

	BtcRegime  string  `json:"btcRegime"`

	TakerFlow  string  `json:"takerFlow"`

	CVDState   string  `json:"cvdState"`

}



type SessionCockpit struct {

	Session         string   `json:"session"`

	Floor           int      `json:"floor"`

	TradesToday     int      `json:"tradesToday"`

	MinTradesPerDay int      `json:"minTradesPerDay"`

	MaxTradesPerDay int      `json:"maxTradesPerDay"`

	TargetTrades    int      `json:"targetTradesPerDay"`

	ActivePlaybooks []string `json:"activePlaybooks"`

	BtcChange5mPct  float64  `json:"btcChange5mPct"`

	Armed           bool     `json:"armed"`

	TradingEnabled  bool     `json:"tradingEnabled"`

	RegimeLabel     string   `json:"regimeLabel"`

	SignalCount     int      `json:"signalCount"`

	NextFloorDrop   string   `json:"nextFloorDrop,omitempty"`

}



type EngineHeartbeat struct {

	LastScanAt        time.Time `json:"lastScanAt"`

	SymbolsScanned    int       `json:"symbolsScanned"`

	Candidates        int       `json:"candidates"`

	AboveFloor        int       `json:"aboveFloor"`

	NearMissCount     int       `json:"nearMissCount"`

	WillFireCount     int       `json:"willFireCount"`

	MaxStrength       int       `json:"maxStrength"`

	MedianStrength    int       `json:"medianStrength"`

	FlatCVDCount      int       `json:"flatCvdCount"`

	MarketDataHealthy bool      `json:"marketDataHealthy"`

	BotState          string    `json:"botState"`

	UniverseSize      int       `json:"universeSize"`

}



type SignalFeedEvent struct {

	At        time.Time `json:"at"`

	Kind      string    `json:"kind"`

	Symbol    string    `json:"symbol,omitempty"`

	Strength  int       `json:"strength,omitempty"`

	Playbook  string    `json:"playbook,omitempty"`

	Message   string    `json:"message"`

}



type PositionLive struct {

	Symbol          string   `json:"symbol"`

	Side            Side     `json:"side"`

	EntryPrice      float64  `json:"entryPrice"`

	MarkPrice       float64  `json:"markPrice"`

	Quantity        float64  `json:"quantity"`

	RemainingQty    float64  `json:"remainingQty"`

	Leverage        int      `json:"leverage"`

	StopPrice       float64  `json:"stopPrice"`

	TrailPrice      float64  `json:"trailPrice,omitempty"`

	TakeProfitPrice float64  `json:"takeProfitPrice"`

	UnrealizedPnL   float64  `json:"unrealizedPnl"`

	RMultiple       float64  `json:"rMultiple"`

	HoldSec         int64    `json:"holdSec"`

	Playbook        string   `json:"playbook"`

	StrengthAtEntry int      `json:"strengthAtEntry"`

	ExitPhase       string   `json:"exitPhase"`

	PeakR           float64  `json:"peakR"`

	RulesArmed      []string `json:"rulesArmed"`

	PartialPct      float64  `json:"partialPctDone"`

	GuardianStatus  string   `json:"guardianStatus"`

	Paper           bool     `json:"paper"`

}



type PlaybookStat struct {

	Playbook    string  `json:"playbook"`

	Trades      int     `json:"trades"`

	WinRate     float64 `json:"winRate"`

	AvgR        float64 `json:"avgR"`

	NetPnL      float64 `json:"netPnl"`

	BestSession string  `json:"bestSession"`

}



type SignalEvent struct {

	ID        int64     `json:"id"`

	Symbol    string    `json:"symbol"`

	Side      string    `json:"side"`

	Playbook  string    `json:"playbook"`

	Session   string    `json:"session"`

	Strength  int       `json:"strength"`

	Rank      int       `json:"rank"`

	Traded    bool      `json:"traded"`

	CreatedAt time.Time `json:"createdAt"`

}


