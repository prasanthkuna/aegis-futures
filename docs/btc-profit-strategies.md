# Core Swing Strategies — BTC / SOL / ETH Catalog & Production Integration

Backtest window: **60 days** per symbol (cached 5m klines, resampled to 15m / 1h / 4h).  
Fees: **4 bps taker + 2 bps slippage**. Split: **70% in-sample / 30% out-of-sample**.  
Prod grid: **$5 risk**, **5× leverage**, **max 4 trades/day** (`-btcprod`).

> **Production rule:** **one playbook per symbol** on **1h** — do **not** stack S4+S14+S11 (COMBO loses on all three cores).

---

## Core trio — per-symbol prod playbook ⭐

| Symbol | Playbook | Variant | RR | Stop | 60d net | OOS | Trades |
|--------|----------|---------|-----|------|---------|-----|--------|
| **BTCUSDT** | S4 Squeeze | **liberal** | 4.0 | 1.4 ATR | **+$98.68** | +$72.17 | 30 |
| **SOLUSDT** | S11 EMA Trend | **liberal** | 3.0 | 1.2 ATR | **+$105.66** | +$69.52 | 71 |
| **ETHUSDT** | S11 EMA Trend | **strict** | 4.0 | 1.8 ATR | **+$84.43** | +$36.14 | 42 |

Shared settings: **1h timeframe**, **$5 risk** (or **2% equity**), **5× leverage**, **max 4 trades/day**.

### Core trio comparison

| | **BTC** | **SOL** | **ETH** |
|--|---------|---------|---------|
| **#1 strategy** | S4 Squeeze liberal | S11 EMA liberal | S11 EMA strict |
| **Net ($5 risk)** | +$98.68 | **+$105.66** | +$84.43 |
| **OOS** | **+$72.17** | +$69.52 | +$36.14 |
| **Trades** | 30 | 71 | 42 |
| **#2 on symbol** | S11 liberal +$52 | S4 liberal +$63 | S11 liberal +$70 |
| **S4 Squeeze liberal** | +$99 ✅ best | +$63 | +$39 (weak) |
| **COMBO book** | −$36 ❌ | −$74 ❌ | −$47 ❌ |

**Ranking by net $:** SOL → BTC → ETH  
**Ranking by OOS:** BTC ≈ SOL → ETH

### ROI on $200 capital (2% = $4/trade, scaled ×0.8 from $5 backtest)

| Symbol | Best playbook | ~60d profit | ~60d ROI | End balance |
|--------|---------------|-------------|----------|-------------|
| SOL | S11 liberal | **~$85** | **~42%** | ~$285 |
| BTC | S4 liberal | **~$79** | **~40%** | ~$279 |
| ETH | S11 strict | **~$68** | **~34%** | ~$268 |

**Compounding rule:** risk = **2% of current equity** each trade; profits increase size automatically.

---

## SOLUSDT — results (`-btcprod` / `-btcrefine`)

### Prod book (1h, $5, 5×, max 4/day)

| Strategy | Variant | Net | OOS | Tr | RR | Stop |
|----------|---------|-----|-----|-----|-----|------|
| **S11 EMA Trend** | **liberal** | **+$105.66** | +$69.52 | 71 | 3.0 | 1.2 |
| S4 Squeeze | liberal | +$63.01 | +$33.23 | 25 | 4.0 | 1.8 |
| S11 EMA Trend | strict | +$58.34 | +$12.30 | 67 | 2.5 | 1.0 |
| S4 Squeeze | strict | +$52.35 | +$28.36 | 18 | 4.0 | 1.8 |
| S14 Donchian | liberal | +$27.86 | +$19.48 | 44 | 3.5 | 1.8 |
| COMBO | — | **−$73.87** | +$39.77 | 87 | — | ❌ |

**SOL notes:**
- Best edge is **S11 liberal**, not S4 (opposite of BTC).
- **15m can work on SOL** (S5 RSI2 +$94 at $5 risk) — noisier; 1h S11 still preferred.
- Report: `backtest/btc-prod-solusdt.txt`

---

## ETHUSDT — results (`-btcprod` / `-btcrefine`)

### Prod book (1h, $5, 5×, max 4/day)

| Strategy | Variant | Net | OOS | Tr | RR | Stop |
|----------|---------|-----|-----|-----|-----|------|
| **S11 EMA Trend** | **strict** | **+$84.43** | +$36.14 | 42 | 4.0 | 1.8 |
| S11 EMA Trend | liberal | +$69.59 | +$31.75 | 63 | 3.5 | 1.4 |
| S4 Squeeze | liberal | +$39.39 | +$12.01 | 27 | 3.5 | 1.4 |
| S14 Donchian | liberal | +$23.22 | +$15.40 | 50 | 2.5 | 1.2 |
| COMBO | — | **−$47.42** | +$13.47 | 83 | — | ❌ |

**Refine highlights (1h, higher sizing):**

| Config | Strategy | Net | OOS | Tr |
|--------|----------|-----|-----|-----|
| $5 / max 2 day | S5 RSI2 | +$177.78 | +$40.94 | 95 |
| $8 / max 1 day | S11 EMA strict | +$145.98 | +$72.27 | 43 |
| $5 / max 2 day | S13 NY Continuation | +$29.96 | **+$53.30** | 25 |

**ETH notes:**
- **S11 strict beats liberal** on ETH (unlike SOL).
- **S4 works but weak** (+$39) — do not use BTC’s S4 as primary on ETH.
- S5 RSI2 highest raw $ but **95 trades** — higher churn; S11 strict is cleaner prod pick.
- Report: `backtest/btc-prod-ethusdt.txt`

---

## Capital & ROI reference ($200) — BTC primary

| Setting | Value |
|---------|--------|
| Capital | **$200** |
| Risk/trade (2%) | **$4** |
| Leverage cap | **5×** → max **$1,000** notional |
| Max trades/day | **4** |

| Metric | BTC S4 liberal (scaled from $5 backtest) |
|--------|-------------------------------------------|
| 60d net PnL | **~$79** |
| 60d ROI | **~40%** |
| End balance | **~$279** |
| Trades | ~30 (~0.5/day) |
| OOS PnL (scaled) | **~$58** |

---

## Tier 1 — Deploy (BTCUSDT)

### 1. S4 Squeeze Breakout — LIBERAL ⭐ PRIMARY

| | |
|--|--|
| **ID** | `S4_SQUEEZE_LIBERAL` |
| **Timeframe** | **1h** (12 × 5m bars) |
| **60d net / OOS** | **+$98.68 / +$72.17** (at $5 risk on $250) |
| **Trades** | 30 |
| **Max trades/day** | **4** |
| **RR** | **4.0** |
| **Stop** | **1.4 × ATR(14)** |
| **PF** | ~1.3+ |
| **Code** | `backtest/btc_lab_strategies.go` → `S4_SQUEEZE_LIBERAL` |

**Entry rules:**
1. ATR(14) at compression: current ATR ≤ 1.18× min ATR over last 20 bars.
2. Price breaks **10-bar** high (long) or low (short).
3. Long: close ≥ EMA21 × 0.998. Short: close ≤ EMA21 × 1.002.

**Why prod:** Best net $, liberal entries beat strict (+$99 vs +$47), OOS strong.

---

### 2. S4 Squeeze — STRICT (backup)

| | |
|--|--|
| **ID** | `S4_SQUEEZE_BREAK` |
| **Timeframe** | 1h |
| **60d net / OOS** | +$47 / +$58 (liberal prod grid, $5 risk) |
| **Trades** | 17 |
| **RR / Stop** | 4.0 / 1.8 ATR |
| **Use when** | Want fewer, higher-conviction trades |

---

### 3. S14 Donchian + Volume — LIBERAL (OOS robust)

| | |
|--|--|
| **ID** | `S14_DONCHIAN_LIBERAL` |
| **Timeframe** | 1h |
| **60d net / OOS** | +$38.52 / **+$60.83** |
| **Trades** | 42 |
| **RR / Stop** | 3.0 / 1.8 ATR |
| **Use when** | Prefer strongest OOS; accept lower net |

**Entry:** 16-bar channel break, volume ≥ 1.15× 16-bar avg, price vs EMA48 aligned.

---

## Tier 2 — Profitable but secondary (1h, $5 risk 5×)

| Rank | Strategy | Net $ | OOS $ | Tr | RR | Stop | Notes |
|------|----------|-------|-------|-----|-----|------|-------|
| 4 | S11 EMA Trend strict | 70.46 | 17.64 | 47 | 2.5 | 1.8 | Weak OOS |
| 5 | S11 EMA Trend liberal | 52.43 | 37.70 | 60 | 4.0 | 1.4 | More trades |
| 6 | S14 Donchian strict | 42.28 | 38.51 | 38 | 3.0 | 1.8 | Solid |
| 7 | S5 RSI2 Snapback | 37.01 | 12.54 | 57 | 3.5 | 2.2 | High trade count |
| 8 | S13 NY Continuation | 17.24 | 7.96 | 30 | 3.0 | 1.0 | Session play |

From **refine run** (`-btcrefine`, $8 risk max 1/day): S4 +$84, S14 +$84, S11 +$58.

---

## Tier 3 — 4h swing (few trades, lower $)

| Strategy | Net $ | OOS $ | Tr | Best for |
|----------|-------|-------|-----|----------|
| S11 EMA Trend | 42.78 | 6.06 | 11 | Low frequency |
| S14 Donchian | 20.42 | **70.59** | 13 | OOS only |
| S4 Squeeze | 22.76 | 11.52 | **4** | Ultra-selective |

---

## Do NOT run (all core symbols)

| Item | BTC | SOL | ETH |
|------|-----|-----|-----|
| **COMBO** S4+S14+S11, 6/day | −$36 | −$74 | −$47 |
| All strategies on **5m** | Loses | Loses | Loses |
| **Aegis playbooks** (`-btcscan`) | 0 winners | — | — |
| Stack multiple playbooks | ❌ | ❌ | ❌ |

Also avoid on BTC: VWAP touch, momentum ignition, deep BB, BB_STRETCH_REVERT.

---

## Alt universe book (separate from core trio)

For **top-30 alts** (not BTC), the only consistent winner in Aegis engine scans:

- **SESSION_BREAKOUT + MEAN_REVERT_VWAP**, floor 58, u30, ~4 trades/day  
- ~**+$48 net** / 60d on 200 symbols (heavy single-symbol concentration on alts)

**Do not port this to BTC** — edges are different.

---

## Backtest commands

```powershell
go build -o .\backtest\aegis-pro.exe .\cmd\backtest\

# Core trio — liberal 1h prod book
.\backtest\aegis-pro.exe -btcprod   -symbol BTCUSDT -days 60
.\backtest\aegis-pro.exe -btcprod   -symbol SOLUSDT -days 60
.\backtest\aegis-pro.exe -btcprod   -symbol ETHUSDT -days 60

# Full TF + leverage grid
.\backtest\aegis-pro.exe -btcrefine -symbol BTCUSDT -days 60
.\backtest\aegis-pro.exe -btcrefine -symbol SOLUSDT -days 60
.\backtest\aegis-pro.exe -btcrefine -symbol ETHUSDT -days 60

# Discovery (single symbol)
.\backtest\aegis-pro.exe -btclab   -symbol BTCUSDT -days 60
```

Reports:
- `backtest/btc-prod-btcusdt.txt`
- `backtest/btc-prod-solusdt.txt`
- `backtest/btc-prod-ethusdt.txt`
- `backtest/btc-refine-*.txt`

Config constants: `config/btc_prod.go`


---

# Engine integration plan

Current live engine is **5m multi-symbol** with score floor + Aegis playbooks (`signal/engine.go`, `signal/playbooks.go`). Core swing needs a **parallel path** per symbol.

## Architecture: two trading modes

```
┌──────────────────────────────────────────────────────────────────┐
│  trading_mode = "alt_scan"    │  trading_mode = "core_swing"     │
├───────────────────────────────┼──────────────────────────────────┤
│  Universe: top 200 alts       │  Universe: BTC / ETH / SOL only  │
│  Candles: 5m                  │  Candles: 1h (resampled)         │
│  Playbooks: MOMENTUM, etc.    │  Per-symbol playbook (see table) │
│  Score floor: 55–78           │  Trigger = binary (no floor)     │
│  Stop: 1.5 ATR (generic)      │  Per-symbol RR + stop ATR        │
│  Risk: $1.25 default          │  Risk: 2% equity, 5x lev         │
└───────────────────────────────┴──────────────────────────────────┘
```

Per-symbol playbook map (from backtest):

| Symbol | Playbook ID | RR | Stop ATR |
|--------|-------------|-----|----------|
| BTCUSDT | `S4_SQUEEZE_LIBERAL` | 4.0 | 1.4 |
| SOLUSDT | `S11_EMA_TREND_LIBERAL` | 3.0 | 1.2 |
| ETHUSDT | `S11_EMA_TREND` (strict) | 4.0 | 1.8 |

## Step-by-step engine changes

### Phase 1 — Data layer (1h bars)

**File:** `market/hub.go`

- Add `Candles1h []Candle` to `SymbolState`.
- On each **closed 5m bar**, when `barIndex % 12 == 0`, append one 1h candle (same logic as `backtest/resampleBars`).
- Keep at least **60** 1h bars for indicators.
- Expose `Hub.Candles1h(symbol)`.

### Phase 2 — Strategy logic

**New file:** `signal/btc_swing.go` (rename to `signal/core_swing.go` when multi-symbol)

- Port `S4_SQUEEZE_LIBERAL`, `S11_EMA_TREND`, `S11_EMA_TREND_LIBERAL` from `backtest/btc_lab_strategies.go`.
- Export evaluators returning `PlaybookResult` with binary trigger.
- Lookup playbook by symbol from config map (BTC→S4 liberal, SOL→S11 liberal, ETH→S11 strict).

### Phase 3 — Rank loop branch

**File:** `signal/engine.go`

```go
// Pseudocode in Rank() / scoreSymbol()
if config.TradingMode == "core_swing" {
    if !isCoreSymbol(sym.Symbol) { return skip }
    pb := EvalCoreSwing(st.Candles1h, sym.Symbol) // picks playbook by symbol
    // strength = 100 if triggered else 0; skip adaptive floor
}
```

- Add `TradingMode string` to `config.Snapshot` + DB `bot_config.trading_mode`.
- When `core_swing`: set `FloorOverride = 0`, restrict universe to BTC/ETH/SOL.

### Phase 4 — Entry & exit sizing

**File:** `engine/engine.go` → `tryEnterSignal`

| Today | Core swing |
|-------|------------|
| `stopDist = atr * 1.5` | `stopDist = atr * symbolStopATR` (see table) |
| Generic exit manager | Fixed **RR** take-profit at entry (per symbol) |
| `RiskUSDForStrength` | `equity * 0.02` (compound) |
| `MaxLeverage` from config | **5** |

**File:** `execution/execution.go` — already has `RiskQuantity(capital, risk, entry, stop, leverage)`.

**File:** `exit/manager.go`

- Add path: per playbook ID, TP = entry ± stopDist×RR, max hold **36h**.

### Phase 5 — Risk & limits

**File:** `risk/engine.go`

- `MaxTradesPerDay = 4` when `core_swing`.
- Daily hard stop: suggest **3× risk** = $12 on $200 / $4 risk.
- **Disable** BTC block gates for core symbols in swing mode.

**File:** `config/btc_prod.go` — already defines prod defaults; wire `BTCProdSnapshot()` on mode switch.

### Phase 6 — API / persistence

**File:** `aegis/api.go`

- Extend `BotConfig` JSON: `tradingMode`, `btcPlaybook`, `riskPct` (default 2).
- `PATCH /config` applies mode + reloads `config.Live`.

**File:** DB migration

```sql
ALTER TABLE bot_config ADD COLUMN trading_mode TEXT DEFAULT 'alt_scan';
ALTER TABLE bot_config ADD COLUMN core_playbook_map JSONB; -- symbol → playbook + rr + stop
ALTER TABLE bot_config ADD COLUMN risk_pct REAL DEFAULT 2.0;
```

### Phase 7 — Engine tick cadence

**File:** `engine/engine.go` → `RankSignalsAt`

- In `core_swing` mode: evaluate on **1h bar close** only (every 12th 5m tick).

---

## Engine file checklist

| File | Action |
|------|--------|
| `market/hub.go` | Add 1h candle resample |
| `signal/btc_swing.go` | **New** — S4 liberal eval |
| `signal/playbooks.go` | Register `BTC_SQUEEZE_LIBERAL` |
| `signal/engine.go` | Mode branch, skip floor for BTC |
| `engine/engine.go` | Playbook-specific stop/TP, 1h tick |
| `exit/manager.go` | Fixed 4R + 36h time stop |
| `risk/engine.go` | 4 trades/day, compound risk % |
| `config/btc_prod.go` | Prod defaults (exists) |
| `config/live.go` | Add `TradingMode`, `RiskPct` |
| `aegis/api.go` | Expose mode in dashboard API |

---

# UI integration plan

Dashboard is **alt-scanner oriented** (`dashboard/app/page.tsx` — universe grid, session floor, 200 symbols). BTC swing mode needs a **focused cockpit**.

## Step-by-step UI changes

### 1. Trading mode toggle

**Files:** `dashboard/components/TopBar.tsx`, `dashboard/lib/types.ts`

- Add badge: **ALT SCAN** | **CORE SWING**.
- Toggle in Ops drawer → `PATCH /api/config { tradingMode: "core_swing" }`.
- When core swing: hide `UniverseScan`; show BTC/ETH/SOL panels.

### 2. Config panel ($200 setup)

**File:** `dashboard/components/OpsDrawer.tsx` (config tab)

| Field | Default | Maps to |
|-------|---------|---------|
| Capital | 200 | `activeCapitalUsd` |
| Risk % | 2 | `riskPct` → riskUSD = capital × pct |
| Max leverage | 5 | `maxLeverage` |
| Max trades/day | 4 | `maxTradesPerDay` |
| Playbook | Per symbol (see trio table) | `corePlaybookMap` |
| Daily stop $ | 12 | `dailyHardStopUsd` |

Use existing `configDraft` / `onSave` flow in `page.tsx`.

### 3. Core strategy status card (new)

**New file:** `dashboard/components/CoreSwingPanel.tsx`

Show per symbol (BTC / ETH / SOL) when `tradingMode === "core_swing"`:

- 1h bar countdown / last close time
- Strategy-specific state (squeeze / EMA trend / channel)
- Last signal: LONG / SHORT / WAIT
- Trades today: 2/4

### 4. Signal board filter

**File:** `dashboard/components/SignalBoard.tsx`

- Core swing: only show **BTCUSDT**, **ETHUSDT**, **SOLUSDT** rows.

### 5. Session strip

**File:** `dashboard/components/SessionStrip.tsx`

- Core swing: show **1h**, per-symbol **RR + stop ATR**, `tradesToday / maxTradesPerDay`.

### 6. Position commander

**File:** `dashboard/components/PositionCommander.tsx`

- Display per-symbol **TP at RR** and **stop ATR** on entry card.
- Hold timer toward **36h** max.

### 7. Metrics rail / ROI

**File:** `dashboard/components/MetricsRail.tsx`

- Add **ROI %** on active capital: `realizedPnl / activeCapital × 100`.
- Add **risk per trade** ($) from config.

### 8. Types & API

**Files:** `dashboard/lib/types.ts`, `dashboard/lib/api.ts`

```ts
export type BotConfig = {
  // existing...
  tradingMode: "alt_scan" | "core_swing";
  corePlaybookMap: Record<string, { playbook: string; rr: number; stopATr: number }>;
  riskPct: number;
};
```

---

## UI layout (BTC swing mode)

```
┌──────────────────────────────────────────────────┐
│ TopBar: BTC SWING ● armed │ $279 equity │ +39%  │
├──────────────────────────────────────────────────┤
│ SessionStrip: 1h │ 2/4 trades │ RR4 │ stop1.4ATR│
├────────────────────┬─────────────────────────────┤
│ BTCStrategyPanel   │ PositionCommander           │
│ squeeze ● active   │ (open pos or flat)          │
├────────────────────┴─────────────────────────────┤
│ SignalBoard (BTCUSDT only) │ SignalFeed           │
├──────────────────────────────────────────────────┤
│ MetricsRail: net PnL, ROI, PF, DD                │
└──────────────────────────────────────────────────┘
```

---

## Enable core swing (live real orders)

Set Encore secrets (or env):

| Secret | Value | Meaning |
|--------|-------|---------|
| `AegisCoreSwing` | `1` | Enable core swing engine (BTC/ETH/SOL 1h) |
| `AegisTradingEnabled` | `1` | **Real orders** (required) |
| `AegisAggressiveMode` | `0` | Start **conservative** ($200, $4 risk) |
| `BinanceAPIKey` / `BinanceAPISecret` | … | Futures API keys |

After a few days of profitable live trades, set `AegisAggressiveMode=1` for $250 / $5 risk.

### Conservative vs aggressive

| | Conservative (start) | Aggressive (later) |
|--|---------------------|-------------------|
| Capital | $200 | $250 |
| Risk/trade | $4 (2%) | $5 (2%) |
| Leverage | 5× | 5× |
| Max trades/day | 4 | 4 |
| Daily stop | $12 | $15 |

### Per-symbol playbooks (90d+180d validated)

| Symbol | Playbook | RR | Stop |
|--------|----------|-----|------|
| BTCUSDT | S4_SQUEEZE_LIBERAL | 4.0 | 1.4 ATR |
| SOLUSDT | S4_SQUEEZE_LIBERAL | 4.0 | 1.8 ATR |
| ETHUSDT | S11_EMA_TREND_STRICT | 4.0 | 1.8 ATR |

### How it works in the engine

- Seeds **800× 5m bars** per core symbol on startup
- Evaluates on **each new 1h bar close** only
- **One position** at a time across BTC/ETH/SOL
- Fixed **RR take-profit** + **36h** time stop + exchange stop order
- Dashboard `/status` returns `tradingMode: "core_swing"`

Validate long horizon anytime:

```powershell
.\backtest\aegis-pro.exe -corevalidate
.\backtest\aegis-pro.exe -sols4check
```

---

## Recommended rollout order

1. Set secrets → deploy with `AegisCoreSwing=1`, conservative sizing
2. Monitor **daily trades** on dashboard + Telegram (`CORE_ENTRY` / `CORE_EXIT`)
3. After **~1–2 weeks** green → `AegisAggressiveMode=1`
4. **UI mode toggle** — ops drawer (optional follow-up)

---

## Quick reference — prod numbers ($200, core trio)

| Symbol | Playbook | RR | Stop | ~60d $ | ~ROI |
|--------|----------|-----|------|--------|------|
| BTCUSDT | S4 Squeeze liberal | 4.0 | 1.4 ATR | ~$79 | ~40% |
| SOLUSDT | S11 EMA liberal | 3.0 | 1.2 ATR | ~$85 | ~42% |
| ETHUSDT | S11 EMA strict | 4.0 | 1.8 ATR | ~$68 | ~34% |

Shared: **1h**, **2% risk**, **5× lev**, **max 4 trades/day**, **36h max hold**.

*Past backtest performance does not guarantee future results. Paper trade before live.*
