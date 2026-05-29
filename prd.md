I validated the key external assumptions before locking the PRD: Binance USDⓈ-M Futures supports 24h rolling ticker stats, all-market ticker streams, and real order placement via `/fapi/v1/order`; Binance warns 24h ticker stats are rolling-window, not UTC-day stats. CoinGlass API is useful for futures context like OI, funding, liquidation and order-flow data, but should be context, not the hot execution trigger. Encore Go is a strong backend choice because it supports APIs, services, DBs, queues, cron, caches, storage, tracing, secrets, and WebSockets. ([Binance Developers][1])

Here is the final PRD.

# PRD: Aegis Futures V1

## Binance USDT-M Futures Order-Flow Momentum Bot

## 1. Product Summary

Aegis Futures V1 is a Binance-only futures trading system that automatically scans high-volume USDT perpetual contracts, identifies order-flow-confirmed momentum setups, places and manages trades, closes positions, tracks true after-fee PnL, and exposes everything through a professional dashboard.

The product is not an AI prediction bot, not a hedge fund strategy zoo, and not a YouTube strategy clone. It is a production-grade execution and validation lab for one focused strategy:

**Top-volume Binance futures order-flow momentum scalping.**

The first live deployment will use a $1,000 account, but only $250 active capital initially. The system must prove clean execution before more capital is unlocked.

---

## 2. Core Philosophy

The bot should be aggressive in automation, measurement, and execution discipline, not reckless in leverage.

The v1 goal is not “10x immediately.”
The v1 goal is:

* Bot opens and closes trades by itself.
* Stop-loss is always placed immediately.
* PnL is tracked after fees and funding.
* No uncontrolled open positions.
* No missing stops.
* No strategy drift.
* Real execution data is collected for 2 weeks.

If the bot cannot survive cleanly with $250 active capital, it has no right to trade $1,000.

---

## 3. Target User

Primary user: solo technical trader-builder who wants to test systematic Binance futures trading with real money, strict safety, and full PnL transparency.

User needs:

* See live PnL clearly.
* Trust that the bot will close trades by itself.
* Know why each trade was entered and exited.
* Pause or kill the bot instantly.
* Start with small real capital.
* Scale only after evidence.

---

## 4. In Scope

### V1 includes

* Binance USDⓈ-M futures only.
* Real mode from day one (mainnet production target).
* Top-volume universe scanner.
* One trading strategy only.
* Order-flow momentum scalping.
* CVD / taker flow confirmation.
* OI/funding/liquidation context from CoinGlass (optional; requires paid API — bot runs without it).
* Post-only limit entries.
* Reduce-only stop and exit orders.
* Full trade lifecycle automation.
* PnL ledger.
* Risk engine.
* Position guardian.
* Kill switch.
* Next.js dashboard.
* Encore Go backend.

### V1 excludes

* Dashboard authentication (solo operator; private network / Encore URL only).
* Paper shadow mode (removed — validate with one testnet smoke test, then mainnet).
* X/news integration.
* AI-generated trading decisions.
* Prediction markets.
* Arbitrage.
* Multi-exchange trading.
* Portfolio margin.
* Copy-trading whales.
* One-second HFT.
* Multiple hedge-fund-style strategies.
* Manual discretionary entries.

---

## 5. Product Decision

Final stack:

```txt
Backend: Encore Go
Frontend: Next.js
Database: Postgres via Encore
Exchange: Binance USDⓈ-M Futures
Market Context: CoinGlass API
Charts: Lightweight Charts
Alerts: Telegram
Deployment: Encore Cloud or VPS/Docker (engine must run in Binance-eligible region)
```

### V1 implementation decisions (locked)

```txt
Auth: none on dashboard APIs (CORS restricted to dashboard origin in prod).
Network: mainnet is default; BINANCE_USE_TESTNET=true only for one dev smoke test.
Paper shadow: removed.
BTC regime: block LONG if BTC 5m change <= -0.40%; block SHORT if >= +0.40%.
Swing structure: 5m breakout vs prior 20-candle high/low (exclude current bar).
CVD: 3-minute rolling taker buy minus sell from Binance aggTrade (not CoinGlass).
Session score (UTC): Asia 00-08 = 0.6, EU 08-14 = 0.85, US 14-22 = 1.0, late 22-00 = 0.75.
CoinGlass: poll every 3 minutes; context score only; endpoints in §7.3.
Testnet: minimal URL switch only; all business logic identical to mainnet.
Trading gate: AEGIS_TRADING_ENABLED=false until operator explicitly enables live orders.
```

Why this stack:

* Encore Go owns the trading engine and long-running services.
* Next.js is only the dashboard and control room.
* Postgres is the truth ledger.
* Binance is the execution source of truth.
* CoinGlass is a derivatives context layer, not an order trigger.

Do not put the trading loop inside Next.js.

---

## 6. Architecture

```txt
┌────────────────────────────────────┐
│ Next.js Dashboard                   │
│ PnL, trades, positions, config      │
│ start / pause / kill switch         │
└────────────────┬───────────────────┘
                 │ HTTP / SSE / WebSocket
                 ▼
┌────────────────────────────────────┐
│ Encore Go Backend                   │
│ APIs, services, cron, DB, secrets   │
└────────────────┬───────────────────┘
                 ▼
┌────────────────────────────────────┐
│ Trading Engine                      │
│ market data, strategy, execution    │
│ risk, guardian, ledger              │
└────────────────┬───────────────────┘
                 ▼
┌────────────────────────────────────┐
│ Binance Futures                     │
│ live data, orders, fills, positions │
└────────────────────────────────────┘

┌────────────────────────────────────┐
│ CoinGlass Context Engine            │
│ OI, funding, liquidation context    │
└────────────────────────────────────┘
```

---

## 7. Backend Services

### 7.1 Market Service

Responsibilities:

* Connect to Binance futures market streams.
* Track top-volume symbols.
* Track price, spread, depth, trades, and candles.
* Build 1m and 5m market state.
* Emit market snapshots.

Data sources:

* Binance all-market ticker stream.
* Binance 24h ticker fallback.
* Binance aggTrade stream.
* Binance bookTicker/depth stream.
* Binance kline streams.

Important rule:

24h Binance ticker is rolling 24h volume, not UTC-day volume. UI should label it as **rolling 24h volume**.

---

### 7.2 Universe Service

Purpose: choose what symbols the bot is allowed to watch and trade.

Rules:

```json
{
  "method": "rolling_24h_quoteVolume",
  "topN": 10,
  "refreshMinutes": 15,
  "alwaysInclude": ["BTCUSDT", "ETHUSDT", "SOLUSDT"],
  "maxNewSymbolsPerRefresh": 2
}
```

Filters:

* USDT perpetuals only.
* Exclude symbols with poor liquidity.
* Exclude symbols with high spread.
* Exclude symbols with abnormal funding risk.
* Exclude symbols where min order size makes risk invalid.

Universe output:

```txt
symbol
rank
quoteVolume24h
spreadBps
fundingRate
volumeSurge
tradable: true/false
reason
```

---

### 7.3 CoinGlass Context Service

Purpose: provide slow derivatives context.

Use CoinGlass for:

* Open interest trend.
* Funding crowding.
* Liquidation heatmap context.
* Long/short imbalance.
* Squeeze-risk awareness.

Do not use CoinGlass for:

* Millisecond execution.
* Direct order trigger.
* Replacing Binance order-flow data.

CoinGlass score:

```txt
+ positive if OI/funding/liquidation context supports momentum
0 neutral
- negative if crowding/liquidation risk is dangerous
```

---

### 7.4 Strategy Service

V1 strategy:

**Binance Order-Flow Momentum Scalper**

Signal stack:

```txt
TradeScore =
  25% volume surge
  25% CVD / taker flow
  20% structure breakout
  15% OI + funding + liquidation context
  10% spread / depth quality
   5% session quality
```

Minimum score:

```txt
Trade allowed only if TradeScore >= 0.78
```

A+ score for future scaling:

```txt
A+ setup if TradeScore >= 0.88
```

No separate strategies in v1.

---

## 8. Trading Rules

### 8.1 Long Setup

Long allowed only when:

```txt
1. Symbol is in top-volume universe or BTC/ETH/SOL.
2. 5m candle breaks recent swing high.
3. 5m volume > 1.8x recent 20-candle average.
4. Taker buy volume dominates recent 1-3 minutes.
5. CVD confirms upward pressure.
6. OI is rising or neutral.
7. Funding is not extremely positive.
8. Spread and depth pass.
9. BTC is not aggressively dumping.
10. Stop distance is valid.
11. TradeScore >= 0.78.
```

### 8.2 Short Setup

Short allowed only when:

```txt
1. Symbol is in top-volume universe or BTC/ETH/SOL.
2. 5m candle breaks recent swing low.
3. 5m volume > 1.8x recent 20-candle average.
4. Taker sell volume dominates recent 1-3 minutes.
5. CVD confirms downward pressure.
6. OI is rising or neutral.
7. Funding is not extremely negative.
8. Spread and depth pass.
9. BTC is not aggressively pumping.
10. Stop distance is valid.
11. TradeScore >= 0.78.
```

---

## 9. Execution Rules

### 9.1 Entry

Default entry:

```txt
post-only limit order
time in force: GTX
entry attempts: 2
entry timeout: 5 seconds each
market entry fallback: disabled in v1
```

If entry does not fill:

```txt
cancel order
record missed trade
do not chase candle
```

### 9.2 Stop-Loss

After entry fill:

```txt
place reduce-only STOP_MARKET immediately
required within 1 second
```

If stop placement fails:

```txt
emergency exit immediately
pause bot
create risk event
send Telegram alert
```

### 9.3 Take Profit

V1:

```txt
single reduce-only take-profit order
optional trailing stop
max trade duration: 30 minutes
```

Partial exits are disabled initially unless symbol quantity rules allow clean partial sizing.

### 9.4 Exit Reasons

Valid exits:

```txt
stop_loss
take_profit
trailing_stop
timeout
signal_invalidated
daily_loss_limit
manual_kill
guardian_emergency_exit
```

---

## 10. Risk Management

Initial capital:

```txt
Account capital: $1,000
Week 1 active capital: $250
Week 2 active capital: $500 if clean
Full active capital: only after validation
```

Week 1 config:

```json
{
  "accountCapitalUsd": 1000,
  "activeCapitalUsd": 250,
  "maxLeverage": 2,
  "riskPerTradeUsd": 1.25,
  "maxOpenPositions": 1,
  "maxTradesPerDay": 6,
  "dailyHardStopUsd": 7.5,
  "weeklyHardStopUsd": 20,
  "maxConsecutiveLosses": 3,
  "cooldownAfterLossMinutes": 20
}
```

Week 2 config if clean:

```json
{
  "activeCapitalUsd": 500,
  "maxLeverage": 2,
  "riskPerTradeUsd": 3.75,
  "maxOpenPositions": 1,
  "maxTradesPerDay": 8,
  "dailyHardStopUsd": 12.5,
  "weeklyHardStopUsd": 35,
  "maxConsecutiveLosses": 3
}
```

Full capital unlock requires:

```txt
no missing stop
no order-state mismatch
no uncontrolled position
30+ closed trades
fees under control
drawdown acceptable
```

---

## 11. Position Guardian

The Position Guardian is the most important safety component.

It continuously verifies:

```txt
Is there an open position?
Is there a stop-loss order?
Is stop quantity correct?
Is take-profit quantity correct?
Are orders reduce-only?
Is actual Binance position equal to internal DB state?
Is unrealized loss within limits?
Has WebSocket disconnected?
Has order state become unknown?
```

Guardian actions:

```txt
pause_new_entries
cancel_stale_orders
replace_missing_stop
emergency_close
trigger_kill_switch
send_alert
write_risk_event
```

Non-negotiable rule:

```txt
No open position may exist without a protective stop.
```

---

## 12. Bot States

```txt
IDLE
SCANNING
SETUP_FOUND
RISK_CHECKING
ORDER_PLACING
ENTRY_PENDING
ENTRY_FILLED
STOP_PLACING
IN_POSITION
EXIT_PENDING
CLOSED
COOLDOWN
PAUSED
ERROR
KILL_SWITCH
```

Every state transition must be logged.

---

## 13. Dashboard Requirements

### 13.1 Command Center

Show:

```txt
mode
bot status
active capital
account balance
available margin
open PnL
realized PnL
net PnL after fees
today PnL
weekly PnL
fees paid
funding paid/received
current drawdown
kill switch status
last trade
open position
```

### 13.2 Setup Radar

For top symbols:

```txt
rank
symbol
rolling 24h quoteVolume
price
spread bps
volume surge
CVD state
taker flow
OI/funding context
CoinGlass context score
session score
TradeScore
decision: trade / watch / skip
reason
```

### 13.3 Open Position View

Show:

```txt
symbol
side
entry price
current price
quantity
leverage
stop price
take-profit price
unrealized PnL
fees so far
R multiple
time in trade
guardian status
```

### 13.4 Closed Trades

Show:

```txt
trade id
symbol
side
entry time
exit time
entry price
exit price
quantity
gross PnL
fees
funding
net PnL
R multiple
exit reason
TradeScore
session
mistake tag
```

### 13.5 Strategy Truth

Show:

```txt
win rate
average win
average loss
profit factor
expectancy per trade
max drawdown
fees as % of gross profit
best symbol
worst symbol
best session
worst session
long vs short performance
post-only fill rate
missed trade count
stop missing incidents
state mismatch incidents
```

Most important metric:

```txt
expectancy after fees
```

---

## 14. Database Tables

Minimum tables:

```txt
bot_runs
bot_config
account_snapshots
symbol_snapshots
setup_scores
orders
fills
positions
trades
pnl_ledger
risk_events
guardian_checks
config_versions
missed_trades
```

### trades table

```txt
id
bot_run_id
mode
symbol
side
entry_time
exit_time
entry_price
exit_price
quantity
leverage
gross_pnl
fees
funding
net_pnl
r_multiple
trade_score
entry_reason
exit_reason
session
config_version
created_at
```

### pnl_ledger table

```txt
id
timestamp
trade_id
balance_before
balance_after
gross_pnl
commission
funding
net_pnl
realized_pnl
unrealized_pnl
source
created_at
```

### risk_events table

```txt
id
timestamp
severity
type
symbol
position_id
message
action_taken
resolved
created_at
```

---

## 15. APIs

### Dashboard APIs

```txt
GET /status
GET /dashboard/summary
GET /radar
GET /positions/open
GET /trades/closed
GET /pnl/daily
GET /pnl/weekly
GET /risk-events
GET /config/current
POST /bot/start
POST /bot/pause
POST /bot/kill
POST /config/update
```

### Internal services

```txt
MarketService
UniverseService
StrategyService
ExecutionService
RiskService
GuardianService
LedgerService
AlertService
CoinGlassService
```

---

## 16. Alerts

Use Telegram for:

```txt
bot started
bot paused
kill switch triggered
entry order placed
entry filled
stop placed
stop placement failed
position closed
daily loss limit hit
order state mismatch
WebSocket disconnect
guardian emergency exit
```

Alert format:

```txt
Aegis Futures Alert
Type: STOP_PLACEMENT_FAILED
Symbol: SOLUSDT
Position: LONG
Action: Emergency exit triggered
Time: 2026-05-30 14:22 UTC
```

---

## 17. Validation Plan (no paper shadow)

### Phase 1: Dry run without orders (mainnet data, trading disabled)

Duration: 1-2 days.

Pass criteria:

```txt
streams stable on mainnet public + private (when keys set)
universe updates correctly
TradeScore generated
dashboard accurate
no DB errors
AEGIS_TRADING_ENABLED=false — no orders placed
```

### Phase 1b: Testnet smoke (optional, once)

Duration: < 1 day.

```txt
BINANCE_USE_TESTNET=true
place one round-trip micro trade with stop
confirm guardian + ledger + Telegram
revert to mainnet
```

### Phase 2: Live $250 active capital (mainnet)

Duration: 7 days.

Pass criteria:

```txt
no missing stops
no guardian emergency bug
no uncontrolled position
no state mismatch unresolved
all trades recorded
PnL ledger correct
```

### Phase 3: Live $500 active capital

Duration: 7 days.

Pass criteria:

```txt
30+ closed trades total
max drawdown within limit
fees not excessive
post-only fill rate understood
strategy not overtrading
```

### Phase 4: Full $1,000 active

Only after clean 14-day validation.

---

## 18. Environment & secrets

```txt
# Encore secrets (backend only)
BINANCE_API_KEY
BINANCE_API_SECRET
BINANCE_USE_TESTNET          # "true" only for smoke test; default "false"
COINGLASS_API_KEY
TELEGRAM_BOT_TOKEN
TELEGRAM_CHAT_ID
AEGIS_TRADING_ENABLED        # "true" to allow live orders
AEGIS_ENV                    # development | production

# Next.js (dashboard/.env.local)
NEXT_PUBLIC_API_BASE_URL     # Encore API URL, e.g. http://localhost:4000
```

Never store exchange keys in Vercel or the browser.

---

## 19. Success Metrics

### System success

```txt
0 missing-stop incidents
0 uncontrolled positions
0 unresolved order-state mismatches
100% closed trades recorded
100% fills reconciled
guardian check running continuously
```

### Trading success

```txt
positive or near-flat net PnL after fees during validation
max drawdown within configured limits
fees < 35% of gross profit
expectancy after fees measured
clear best/worst symbols identified
```

### Product success

```txt
dashboard shows true PnL
user can understand every trade
bot can be paused/killed safely
capital scaling decision is evidence-based
```

---

## 20. CoinGlass endpoints (v4)

Base: `https://open-api-v4.coinglass.com` — header `CG-API-KEY`.

```txt
/api/futures/open-interest/aggregated-history
/api/futures/funding-rate/history
/api/futures/liquidation/aggregated-history
/api/futures/global-long-short-account-ratio/history
```

---

## 21. Non-Negotiable Safety Rules

```txt
1. No open position without stop-loss.
2. No market entry fallback in v1.
3. No averaging down.
4. No manual revenge trades.
5. No more than one open position in initial phase.
6. No trading after daily hard stop.
7. No trading after 3 consecutive losses.
8. No trading if Binance state and internal state mismatch.
9. No trading if market data stream is unhealthy.
10. No scaling capital until execution is clean.
```

---

## 22. Future Roadmap

### V2

```txt
increase active capital
add A+ setup sizing
add liquidation magnet scoring
add more advanced CoinGlass filters
add correlation filter
add session-specific thresholds
```

### V3

```txt
news/X ignition module
selective market entry fallback for A+ setups
multi-position support
portfolio exposure engine
advanced backtesting harness
```

### V4

```txt
multi-exchange context
hedging module
strategy marketplace
desktop Tauri terminal
institutional-style analytics
```

---

## 23. Final Build Decision

Build one clean strategy first.

Do not add hedge fund strategy catalog now.
Do not add news now.
Do not add arbitrage now.
Do not add AI trade decisions now.

The v1 product is:

```txt
A production-grade Binance futures bot that trades one order-flow momentum strategy, manages the full order lifecycle, tracks true PnL, and protects capital with strict guardian logic.
```

The only acceptable v1 outcome is a system that tells the truth.

Profit comes later. Truth comes first.

[1]: https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/24hr-Ticker-Price-Change-Statistics?utm_source=chatgpt.com "24hr Ticker Price Change Statistics | Binance Open Platform"
