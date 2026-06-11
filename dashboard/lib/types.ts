export type Summary = {
  mode: string;
  botStatus: string;
  activeCapitalUsd: number;
  accountBalance: number;
  availableMargin: number;
  openPnl: number;
  realizedPnl: number;
  netPnlAfterFees: number;
  todayPnl: number;
  weeklyPnl: number;
  feesPaid: number;
  fundingPaid: number;
  currentDrawdown: number;
  killSwitchActive: boolean;
  lastTradeSymbol: string;
  hasOpenPosition: boolean;
  tradingEnabled: boolean;
  testnet: boolean;
};

export type ScoreComponents = {
  volume: number;
  cvd: number;
  structure: number;
  context: number;
  depth: number;
  session: number;
};

export type RadarRegime = {
  label: string;
  summary: string;
  tradeCount: number;
  watchCount: number;
  skipCount: number;
  maxScore: number;
  medianSurge: number;
  btcChange5mPct: number;
};

export type ClosedTrade = {
  id: number;
  symbol: string;
  side: string;
  entryTime: string;
  exitTime?: string;
  entryPrice: number;
  exitPrice?: number;
  quantity: number;
  grossPnl: number;
  fees: number;
  funding: number;
  netPnl: number;
  rMultiple: number;
  exitReason: string;
  tradeScore: number;
  session?: string;
};

export type RiskEvent = {
  id: number;
  timestamp: string;
  severity: string;
  type: string;
  symbol?: string;
  message: string;
  actionTaken?: string;
  resolved: boolean;
};

export type BotConfig = {
  accountCapitalUsd: number;
  activeCapitalUsd: number;
  maxLeverage: number;
  riskPerTradeUsd: number;
  maxOpenPositions: number;
  maxTradesPerDay: number;
  dailyHardStopUsd: number;
  weeklyHardStopUsd: number;
  minTradeScore: number;
};

export type StrategyTruth = {
  winRate: number;
  avgWin: number;
  avgLoss: number;
  profitFactor: number;
  expectancyAfterFees: number;
  maxDrawdown: number;
  closedTradeCount: number;
};

export type BotStatus = {
  state: string;
  tradingEnabled: boolean;
  paused: boolean;
  armed: boolean;
  universeSize: number;
};

export type ProSignal = {
  rank: number;
  symbol: string;
  side: string;
  strength: number;
  playbook: string;
  session: string;
  price: number;
  spreadBps: number;
  quoteVolume24h: number;
  isCore: boolean;
  willFire: boolean;
  components: ScoreComponents;
  extra: {
    vwapDevPct: number;
    atr: number;
    ema9: number;
    faTilt: number;
    btcRegime: string;
    takerFlow: string;
    cvdState: string;
  };
};

export type SessionCockpit = {
  session: string;
  floor: number;
  tradesToday: number;
  minTradesPerDay: number;
  maxTradesPerDay: number;
  targetTradesPerDay: number;
  activePlaybooks: string[];
  btcChange5mPct: number;
  armed: boolean;
  tradingEnabled: boolean;
  regimeLabel: string;
  signalCount: number;
  nextFloorDrop?: string;
};

export type SignalsData = {
  signals: ProSignal[];
  session: SessionCockpit;
  floor: number;
  regime: RadarRegime;
};

export type PositionLive = {
  symbol: string;
  side: string;
  entryPrice: number;
  markPrice: number;
  quantity: number;
  remainingQty: number;
  leverage: number;
  stopPrice: number;
  trailPrice?: number;
  takeProfitPrice: number;
  unrealizedPnl: number;
  rMultiple: number;
  holdSec: number;
  playbook: string;
  strengthAtEntry: number;
  exitPhase: string;
  peakR: number;
  rulesArmed: string[];
  partialPctDone: number;
  guardianStatus: string;
};

export type PositionLiveData = {
  hasPosition: boolean;
  position: PositionLive;
};

export type DashboardData = {
  summary: Summary;
  signals: SignalsData | null;
  session: SessionCockpit | null;
  positionLive: PositionLiveData | null;
  trades: ClosedTrade[];
  truth: StrategyTruth;
  events: RiskEvent[];
  config: BotConfig;
  status: BotStatus;
};
