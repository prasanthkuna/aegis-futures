"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import {
  fmtDuration,
  fmtNum,
  fmtPct,
  fmtScore,
  fmtTime,
  fmtUsd,
  pnlClass,
} from "@/lib/format";
import { RadarPanel } from "@/components/RadarPanel";
import { normalizeRadarItem } from "@/lib/radar-utils";
import type {
  BotConfig,
  ClosedTrade,
  DashboardData,
  OpenPosition,
  RadarData,
  RiskEvent,
  StrategyTruth,
  Summary,
} from "@/lib/types";

const POLL_MS = 5000;

const emptyRadar: RadarData = {
  items: [],
  meta: {
    minTradeScore: 0.78,
    watchMinScore: 0.66,
    aPlusTradeScore: 0.88,
    armed: false,
    tradingEnabled: false,
    paused: false,
    killSwitch: false,
    tradesToday: 0,
    maxTradesPerDay: 6,
    openPositions: 0,
    maxOpenPositions: 1,
    todayPnl: 0,
    dailyHardStopUsd: 0,
  },
  regime: {
    label: "—",
    summary: "",
    tradeCount: 0,
    watchCount: 0,
    skipCount: 0,
    maxScore: 0,
    medianSurge: 0,
    btcChange5mPct: 0,
  },
};

const emptyTruth: StrategyTruth = {
  winRate: 0,
  avgWin: 0,
  avgLoss: 0,
  profitFactor: 0,
  expectancyPerTrade: 0,
  expectancyAfterFees: 0,
  maxDrawdown: 0,
  feesPctOfGrossProfit: 0,
  bestSymbol: "",
  worstSymbol: "",
  bestSession: "",
  worstSession: "",
  longPnl: 0,
  shortPnl: 0,
  postOnlyFillRate: 0,
  missedTradeCount: 0,
  stopMissingIncidents: 0,
  stateMismatchCount: 0,
  closedTradeCount: 0,
};

async function loadAll(): Promise<{ data: DashboardData; warnings: string[] }> {
  const warnings: string[] = [];
  async function get<T>(path: string, fallback: T): Promise<T> {
    try {
      return await api<T>(path);
    } catch {
      warnings.push(path);
      return fallback;
    }
  }

  const [
    summary,
    radarRes,
    posRes,
    tradesRes,
    truth,
    eventsRes,
    config,
    pnlDaily,
    pnlWeekly,
    status,
  ] = await Promise.all([
    get("/dashboard/summary", null as unknown as Summary),
    get("/radar", emptyRadar),
    get("/positions/open", { positions: [] as OpenPosition[] }),
    get("/trades/closed", { trades: [] as ClosedTrade[] }),
    get("/dashboard/strategy-truth", emptyTruth),
    get("/risk-events", { events: [] as RiskEvent[] }),
    get("/config/current", null as unknown as BotConfig),
    get("/pnl/daily", { points: [] }),
    get("/pnl/weekly", { points: [] }),
    get("/status", {
      state: "—",
      tradingEnabled: false,
      paused: false,
      armed: false,
      universeSize: 0,
    }),
  ]);

  if (!summary) {
    throw new Error("Could not load dashboard summary");
  }

  const minScore =
    radarRes.meta?.minTradeScore ?? config?.minTradeScore ?? 0.78;
  const radarItems = (radarRes.items ?? []).map((item) =>
    normalizeRadarItem(item, minScore)
  );

  return {
    data: {
      summary,
      radar: {
        items: radarItems,
        meta: radarRes.meta ?? emptyRadar.meta,
        regime: radarRes.regime ?? emptyRadar.regime,
      },
      positions: posRes.positions ?? [],
      trades: tradesRes.trades ?? [],
      truth,
      events: eventsRes.events ?? [],
      config:
        config ??
        ({
          accountCapitalUsd: 0,
          activeCapitalUsd: 0,
          maxLeverage: 0,
          riskPerTradeUsd: 0,
          maxOpenPositions: 0,
          maxTradesPerDay: 0,
          dailyHardStopUsd: 0,
          weeklyHardStopUsd: 0,
          minTradeScore: 0,
        } satisfies BotConfig),
      pnlDaily: pnlDaily.points ?? [],
      pnlWeekly: pnlWeekly.points ?? [],
      status,
    },
    warnings,
  };
}

export default function Home() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [busy, setBusy] = useState<string | null>(null);
  const [configDraft, setConfigDraft] = useState({
    active: "",
    risk: "",
    minScore: "",
    maxTrades: "",
    dailyStop: "",
    weeklyStop: "",
    maxLev: "",
  });

  const refresh = useCallback(async () => {
    try {
      const { data: d, warnings: w } = await loadAll();
      setData(d);
      setWarnings(w);
      setConfigDraft({
        active: String(d.config.activeCapitalUsd),
        risk: String(d.config.riskPerTradeUsd),
        minScore: String(d.config.minTradeScore),
        maxTrades: String(d.config.maxTradesPerDay),
        dailyStop: String(d.config.dailyHardStopUsd),
        weeklyStop: String(d.config.weeklyHardStopUsd),
        maxLev: String(d.config.maxLeverage),
      });
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "load failed");
    }
  }, []);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, POLL_MS);
    return () => clearInterval(id);
  }, [refresh]);

  async function botAction(path: string) {
    setBusy(path);
    try {
      await api(path, { method: "POST" });
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "action failed");
    } finally {
      setBusy(null);
    }
  }

  async function saveConfig() {
    const active = parseFloat(configDraft.active);
    const risk = parseFloat(configDraft.risk);
    let minScore = parseFloat(configDraft.minScore);
    const maxTrades = parseInt(configDraft.maxTrades, 10);
    const dailyStop = parseFloat(configDraft.dailyStop);
    const weeklyStop = parseFloat(configDraft.weeklyStop);
    const maxLev = parseInt(configDraft.maxLev, 10);
    if (Number.isNaN(active) || Number.isNaN(risk) || Number.isNaN(minScore)) return;
    if (minScore > 1) minScore = minScore / 100;
    setBusy("config");
    try {
      await api("/config/update", {
        method: "POST",
        body: JSON.stringify({
          activeCapitalUsd: active,
          riskPerTradeUsd: risk,
          minTradeScore: minScore,
          maxTradesPerDay: Number.isNaN(maxTrades) ? undefined : maxTrades,
          dailyHardStopUsd: Number.isNaN(dailyStop) ? undefined : dailyStop,
          weeklyHardStopUsd: Number.isNaN(weeklyStop) ? undefined : weeklyStop,
          maxLeverage: Number.isNaN(maxLev) ? undefined : maxLev,
        }),
      });
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "config update failed");
    } finally {
      setBusy(null);
    }
  }

  const s = data?.summary;
  const truth = data?.truth;

  return (
    <main>
      <header>
        <div>
          <h1>Aegis Futures</h1>
          <p className="subtitle">
            {data?.status.state ?? "—"}
            {data?.status.paused ? " · paused" : ""}
            {data?.status.armed ? " · armed (can trade)" : " · not armed"}
            {" · "}
            {data?.status.universeSize ?? 0} symbols ·{" "}
            {data?.summary?.testnet ? "testnet" : "mainnet"} · every{" "}
            {POLL_MS / 1000}s
          </p>
        </div>
        <div className="actions">
          <button
            className="start"
            disabled={!!busy}
            onClick={() => botAction("/bot/start")}
          >
            {busy === "/bot/start" ? "…" : "Start"}
          </button>
          <button
            className="pause"
            disabled={!!busy}
            onClick={() => botAction("/bot/pause")}
          >
            {busy === "/bot/pause" ? "…" : "Pause"}
          </button>
          <button
            className="kill"
            disabled={!!busy}
            onClick={() => botAction("/bot/kill")}
          >
            {busy === "/bot/kill" ? "…" : "Kill"}
          </button>
        </div>
      </header>

      {error && <div className="banner error">{error}</div>}
      {warnings.length > 0 && !error && (
        <div className="banner warn">
          Some API endpoints unavailable (deploy backend): {warnings.join(", ")}
        </div>
      )}
      {data?.status && !error && (
        <div
          className={`banner ${data.status.armed ? "ok" : data.status.paused ? "warn" : "info"}`}
        >
          <strong>Start</strong> clears pause and returns to scanning. Live orders
          require <strong>AegisTradingEnabled=true</strong> in Encore secrets
          {data.status.tradingEnabled
            ? " (secret is on)"
            : " (secret is off — no entries)"}
          .
        </div>
      )}

      {s && (
        <section>
          <h2>Command center</h2>
          <div className="grid">
            <Card label="Mode" value={s.mode} />
            <Card label="Bot status" value={s.botStatus} highlight />
            <Card label="Active capital" value={fmtUsd(s.activeCapitalUsd, 0)} />
            <Card
              label="Account balance"
              value={fmtUsd(s.accountBalance)}
              hint="0 if Binance IP whitelist blocks Encore egress"
            />
            <Card label="Available margin" value={fmtUsd(s.availableMargin)} />
            <Card
              label="Open PnL"
              value={fmtUsd(s.openPnl)}
              className={pnlClass(s.openPnl)}
            />
            <Card
              label="Realized PnL"
              value={fmtUsd(s.realizedPnl)}
              className={pnlClass(s.realizedPnl)}
            />
            <Card
              label="Net after fees"
              value={fmtUsd(s.netPnlAfterFees)}
              className={pnlClass(s.netPnlAfterFees)}
            />
            <Card
              label="Today PnL"
              value={fmtUsd(s.todayPnl)}
              className={pnlClass(s.todayPnl)}
            />
            <Card
              label="Weekly PnL"
              value={fmtUsd(s.weeklyPnl)}
              className={pnlClass(s.weeklyPnl)}
            />
            <Card label="Fees paid" value={fmtUsd(s.feesPaid)} />
            <Card
              label="Funding"
              value={fmtUsd(s.fundingPaid)}
              className={pnlClass(-s.fundingPaid)}
            />
            <Card label="Drawdown" value={fmtUsd(s.currentDrawdown)} />
            <Card
              label="Kill switch"
              value={s.killSwitchActive ? "ACTIVE" : "off"}
              className={s.killSwitchActive ? "neg" : ""}
            />
            <Card label="Last trade" value={s.lastTradeSymbol || "—"} />
            <Card
              label="Open position"
              value={s.hasOpenPosition ? "yes" : "flat"}
            />
            <Card label="Trading" value={s.tradingEnabled ? "ON" : "OFF"} />
          </div>
        </section>
      )}

      {data && <RadarPanel radar={data.radar} />}

      <section>
        <h2>Open position</h2>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Symbol</th>
                <th>Side</th>
                <th>Entry</th>
                <th>Mark</th>
                <th>Qty</th>
                <th>Lev</th>
                <th>Stop</th>
                <th>TP</th>
                <th>Unrealized</th>
                <th>R</th>
                <th>Time</th>
                <th>Guardian</th>
              </tr>
            </thead>
            <tbody>
              {(data?.positions ?? []).map((p) => (
                <tr key={p.symbol}>
                  <td>{p.symbol}</td>
                  <td>{p.side}</td>
                  <td>{fmtNum(p.entryPrice, 4)}</td>
                  <td>{fmtNum(p.currentPrice, 4)}</td>
                  <td>{fmtNum(p.quantity, 4)}</td>
                  <td>{p.leverage}x</td>
                  <td>{fmtNum(p.stopPrice, 4)}</td>
                  <td>{fmtNum(p.takeProfitPrice, 4)}</td>
                  <td className={pnlClass(p.unrealizedPnl)}>
                    {fmtUsd(p.unrealizedPnl)}
                  </td>
                  <td>{fmtNum(p.rMultiple, 2)}R</td>
                  <td>{fmtDuration(p.timeInTradeSec)}</td>
                  <td className={p.guardianStatus === "active" ? "pos" : "neg"}>
                    {p.guardianStatus}
                  </td>
                </tr>
              ))}
              {!data?.positions.length && (
                <tr>
                  <td colSpan={12} className="empty">
                    No open position
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      <section>
        <h2>Closed trades</h2>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Symbol</th>
                <th>Side</th>
                <th>Entry</th>
                <th>Exit</th>
                <th>Entry px</th>
                <th>Exit px</th>
                <th>Qty</th>
                <th>Gross</th>
                <th>Fees</th>
                <th>Funding</th>
                <th>Net</th>
                <th>R</th>
                <th>Exit</th>
                <th>Score</th>
                <th>Session</th>
              </tr>
            </thead>
            <tbody>
              {(data?.trades ?? []).map((t) => (
                <tr key={t.id}>
                  <td>{t.id}</td>
                  <td>{t.symbol}</td>
                  <td>{t.side}</td>
                  <td>{fmtTime(t.entryTime)}</td>
                  <td>{fmtTime(t.exitTime)}</td>
                  <td>{fmtNum(t.entryPrice, 4)}</td>
                  <td>{t.exitPrice != null ? fmtNum(t.exitPrice, 4) : "—"}</td>
                  <td>{fmtNum(t.quantity, 4)}</td>
                  <td className={pnlClass(t.grossPnl)}>{fmtUsd(t.grossPnl)}</td>
                  <td>{fmtUsd(t.fees)}</td>
                  <td>{fmtUsd(t.funding)}</td>
                  <td className={pnlClass(t.netPnl)}>{fmtUsd(t.netPnl)}</td>
                  <td>{fmtNum(t.rMultiple, 2)}R</td>
                  <td>{t.exitReason || "—"}</td>
                  <td>{fmtNum(t.tradeScore, 2)}</td>
                  <td>{t.session ?? "—"}</td>
                </tr>
              ))}
              {!data?.trades.length && (
                <tr>
                  <td colSpan={16} className="empty">
                    No closed trades yet
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      {truth && (
        <section>
          <h2>Strategy truth</h2>
          <div className="grid">
            <Card
              label="Expectancy (after fees)"
              value={fmtUsd(truth.expectancyAfterFees, 3)}
              className={pnlClass(truth.expectancyAfterFees)}
              highlight
            />
            <Card label="Win rate" value={fmtPct(truth.winRate)} />
            <Card label="Avg win" value={fmtUsd(truth.avgWin)} className="pos" />
            <Card label="Avg loss" value={fmtUsd(truth.avgLoss)} className="neg" />
            <Card label="Profit factor" value={fmtNum(truth.profitFactor, 2)} />
            <Card label="Max drawdown" value={fmtUsd(truth.maxDrawdown)} />
            <Card
              label="Fees % of gross profit"
              value={`${fmtNum(truth.feesPctOfGrossProfit, 1)}%`}
            />
            <Card label="Best symbol" value={truth.bestSymbol || "—"} />
            <Card label="Worst symbol" value={truth.worstSymbol || "—"} />
            <Card label="Best session" value={truth.bestSession || "—"} />
            <Card label="Worst session" value={truth.worstSession || "—"} />
            <Card
              label="Long PnL"
              value={fmtUsd(truth.longPnl)}
              className={pnlClass(truth.longPnl)}
            />
            <Card
              label="Short PnL"
              value={fmtUsd(truth.shortPnl)}
              className={pnlClass(truth.shortPnl)}
            />
            <Card
              label="Post-only fill rate"
              value={fmtPct(truth.postOnlyFillRate)}
            />
            <Card label="Missed trades" value={String(truth.missedTradeCount)} />
            <Card
              label="Stop incidents"
              value={String(truth.stopMissingIncidents)}
            />
            <Card
              label="State mismatches"
              value={String(truth.stateMismatchCount)}
            />
            <Card
              label="Closed trades"
              value={String(truth.closedTradeCount)}
            />
          </div>
        </section>
      )}

      <div className="split">
        <section>
          <h2>Risk events</h2>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Sev</th>
                  <th>Type</th>
                  <th>Symbol</th>
                  <th>Message</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                {(data?.events ?? []).map((e) => (
                  <tr key={e.id}>
                    <td>{fmtTime(e.timestamp)}</td>
                    <td className={`sev ${e.severity}`}>{e.severity}</td>
                    <td>{e.type}</td>
                    <td>{e.symbol ?? "—"}</td>
                    <td className="reason">{e.message}</td>
                    <td>{e.actionTaken ?? "—"}</td>
                  </tr>
                ))}
                {!data?.events.length && (
                  <tr>
                    <td colSpan={6} className="empty">
                      No risk events
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </section>

        <section>
          <h2>Config & limits</h2>
          {data?.config && (
            <div className="config-panel">
              <dl>
                <dt>Account capital</dt>
                <dd>{fmtUsd(data.config.accountCapitalUsd, 0)}</dd>
                <dt>Max leverage</dt>
                <dd>{data.config.maxLeverage}x</dd>
                <dt>Max open positions</dt>
                <dd>{data.config.maxOpenPositions}</dd>
                <dt>Max trades / day</dt>
                <dd>{data.config.maxTradesPerDay}</dd>
                <dt>Daily hard stop</dt>
                <dd>{fmtUsd(data.config.dailyHardStopUsd)}</dd>
                <dt>Weekly hard stop</dt>
                <dd>{fmtUsd(data.config.weeklyHardStopUsd)}</dd>
                <dt>Min trade score</dt>
                <dd>{fmtScore(data.config.minTradeScore)}</dd>
              </dl>
              <div className="config-edit">
                <label>
                  Active capital (USD)
                  <input
                    value={configDraft.active}
                    onChange={(e) =>
                      setConfigDraft((d) => ({ ...d, active: e.target.value }))
                    }
                  />
                </label>
                <label>
                  Risk per trade (USD)
                  <input
                    value={configDraft.risk}
                    onChange={(e) =>
                      setConfigDraft((d) => ({ ...d, risk: e.target.value }))
                    }
                  />
                </label>
                <label>
                  Min trade score (0.78 or 78)
                  <input
                    value={configDraft.minScore}
                    onChange={(e) =>
                      setConfigDraft((d) => ({ ...d, minScore: e.target.value }))
                    }
                  />
                </label>
                <label>
                  Max trades / day
                  <input
                    value={configDraft.maxTrades}
                    onChange={(e) =>
                      setConfigDraft((d) => ({
                        ...d,
                        maxTrades: e.target.value,
                      }))
                    }
                  />
                </label>
                <label>
                  Daily hard stop (USD)
                  <input
                    value={configDraft.dailyStop}
                    onChange={(e) =>
                      setConfigDraft((d) => ({
                        ...d,
                        dailyStop: e.target.value,
                      }))
                    }
                  />
                </label>
                <label>
                  Weekly hard stop (USD)
                  <input
                    value={configDraft.weeklyStop}
                    onChange={(e) =>
                      setConfigDraft((d) => ({
                        ...d,
                        weeklyStop: e.target.value,
                      }))
                    }
                  />
                </label>
                <label>
                  Max leverage
                  <input
                    value={configDraft.maxLev}
                    onChange={(e) =>
                      setConfigDraft((d) => ({ ...d, maxLev: e.target.value }))
                    }
                  />
                </label>
                <button
                  className="start"
                  disabled={!!busy}
                  onClick={saveConfig}
                >
                  {busy === "config" ? "Saving…" : "Save config"}
                </button>
              </div>
            </div>
          )}
        </section>
      </div>

      <section>
        <h2>PnL history</h2>
        <div className="split">
          <PnLTable title="Daily" rows={data?.pnlDaily ?? []} />
          <PnLTable title="Weekly" rows={data?.pnlWeekly ?? []} />
        </div>
      </section>
    </main>
  );
}

function Card({
  label,
  value,
  hint,
  className = "",
  highlight,
}: {
  label: string;
  value: string;
  hint?: string;
  className?: string;
  highlight?: boolean;
}) {
  return (
    <div className={`card ${className} ${highlight ? "highlight" : ""}`}>
      <label>{label}</label>
      <strong>{value}</strong>
      {hint && <span className="hint">{hint}</span>}
    </div>
  );
}

function PnLTable({
  title,
  rows,
}: {
  title: string;
  rows: { period: string; netPnl: number }[];
}) {
  return (
    <div className="table-wrap">
      <h3>{title}</h3>
      <table>
        <thead>
          <tr>
            <th>Period</th>
            <th>Net PnL</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.period}>
              <td>{r.period}</td>
              <td className={pnlClass(r.netPnl)}>{fmtUsd(r.netPnl)}</td>
            </tr>
          ))}
          {!rows.length && (
            <tr>
              <td colSpan={2} className="empty">
                No data
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
