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

export type RadarItem = {
  rank: number;
  symbol: string;
  quoteVolume24h: number;
  price: number;
  spreadBps: number;
  volumeSurge: number;
  cvdState: string;
  takerFlow: string;
  oiFundingContext: string;
  coinGlassScore: number;
  sessionScore: number;
  tradeScore: number;
  decision: string;
  reason: string;
};

export type OpenPosition = {
  symbol: string;
  side: string;
  entryPrice: number;
  currentPrice: number;
  quantity: number;
  leverage: number;
  stopPrice: number;
  takeProfitPrice: number;
  unrealizedPnl: number;
  feesSoFar: number;
  rMultiple: number;
  timeInTradeSec: number;
  guardianStatus: string;
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
  mistakeTag?: string;
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
  expectancyPerTrade: number;
  expectancyAfterFees: number;
  maxDrawdown: number;
  feesPctOfGrossProfit: number;
  bestSymbol: string;
  worstSymbol: string;
  bestSession: string;
  worstSession: string;
  longPnl: number;
  shortPnl: number;
  postOnlyFillRate: number;
  missedTradeCount: number;
  stopMissingIncidents: number;
  stateMismatchCount: number;
  closedTradeCount: number;
};

export type PnLPoint = { period: string; netPnl: number };

export type BotStatus = {
  state: string;
  tradingEnabled: boolean;
  paused: boolean;
  armed: boolean;
  universeSize: number;
};

export type DashboardData = {
  summary: Summary;
  radar: RadarItem[];
  positions: OpenPosition[];
  trades: ClosedTrade[];
  truth: StrategyTruth;
  events: RiskEvent[];
  config: BotConfig;
  pnlDaily: PnLPoint[];
  pnlWeekly: PnLPoint[];
  status: BotStatus;
};
