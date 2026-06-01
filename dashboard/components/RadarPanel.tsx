"use client";

import { useMemo, useState } from "react";
import {
  filterRadarItems,
  gateIcon,
  normalizeRadarItem,
  sortRadarItems,
  weakestLabel,
  type FilterPreset,
  type SortMode,
} from "@/lib/radar-utils";
import { fmtNum, fmtPct, fmtScore, fmtUsd } from "@/lib/format";
import type { RadarData } from "@/lib/types";

type Props = {
  radar: RadarData;
};

const emptyMeta: RadarData["meta"] = {
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
};

const emptyRegime: RadarData["regime"] = {
  label: "—",
  summary: "",
  tradeCount: 0,
  watchCount: 0,
  skipCount: 0,
  maxScore: 0,
  medianSurge: 0,
  btcChange5mPct: 0,
};

function ComponentBars({
  c,
  minScore,
}: {
  c: RadarData["items"][0]["components"];
  minScore: number;
}) {
  const bars = [
    { label: "Vol", v: c.volume, w: "25%" },
    { label: "CVD", v: c.cvd, w: "25%" },
    { label: "Str", v: c.structure, w: "20%" },
    { label: "Ctx", v: c.context, w: "15%" },
    { label: "Dep", v: c.depth, w: "10%" },
    { label: "Ses", v: c.session, w: "5%" },
  ];
  return (
    <div className="comp-bars" title={`Min trade score ${minScore}`}>
      {bars.map((b) => (
        <div key={b.label} className="comp-bar-wrap">
          <span className="comp-label">{b.label}</span>
          <div className="comp-track">
            <div
              className={`comp-fill ${b.v >= minScore ? "hot" : ""}`}
              style={{ width: `${Math.round(b.v * 100)}%` }}
            />
          </div>
        </div>
      ))}
    </div>
  );
}

export function RadarPanel({ radar }: Props) {
  const [sortMode, setSortMode] = useState<SortMode>("actionability");
  const [filter, setFilter] = useState<FilterPreset>("all");

  const meta = radar.meta ?? emptyMeta;
  const regime = radar.regime ?? emptyRegime;
  const items = useMemo(
    () => (radar.items ?? []).map((item) => normalizeRadarItem(item, meta.minTradeScore)),
    [radar.items, meta.minTradeScore]
  );

  const rows = useMemo(() => {
    const filtered = filterRadarItems(items, filter);
    return sortRadarItems(filtered, sortMode);
  }, [items, filter, sortMode]);

  return (
    <section className="radar-section">
      <div className="radar-head">
        <h2>Setup radar</h2>
        <p className="radar-sub">
          Threshold {meta.minTradeScore.toFixed(2)} · watch ≥
          {meta.watchMinScore.toFixed(2)} · A+ {meta.aPlusTradeScore.toFixed(2)}
        </p>
      </div>

      <div className={`regime-banner regime-${(regime.label || "unknown").toLowerCase()}`}>
        <strong>REGIME: {regime.label}</strong>
        <span>{regime.summary}</span>
        <span className="regime-btc">
          BTC 5m: {regime.btcChange5mPct >= 0 ? "+" : ""}
          {regime.btcChange5mPct.toFixed(2)}%
        </span>
      </div>

      <div className="radar-meta-row">
        <span className={meta.armed ? "armed-yes" : "armed-no"}>
          {meta.armed ? "ARMED" : "NOT ARMED"}
        </span>
        <span>
          Trades {meta.tradesToday}/{meta.maxTradesPerDay}
        </span>
        <span>
          Slots {meta.openPositions}/{meta.maxOpenPositions}
        </span>
        <span>Today PnL {fmtUsd(meta.todayPnl)}</span>
        {meta.killSwitch && <span className="neg">KILL SWITCH</span>}
        {meta.paused && <span className="warn">PAUSED</span>}
        {!meta.tradingEnabled && <span className="muted">Trading OFF</span>}
      </div>

      <div className="radar-controls">
        <label>
          Sort
          <select
            value={sortMode}
            onChange={(e) => setSortMode(e.target.value as SortMode)}
          >
            <option value="actionability">Actionability</option>
            <option value="gap">Closest to trade</option>
            <option value="delta">Score Δ (refresh)</option>
            <option value="score">Raw score</option>
            <option value="surge">Volume surge</option>
            <option value="volume">24h volume</option>
            <option value="spread">Tight spread</option>
          </select>
        </label>
        <div className="filter-chips">
          {(
            [
              ["all", "All"],
              ["actionable", "Trade + watch"],
              ["core", "BTC/ETH/SOL"],
              ["rotators", "Rotators"],
              ["btc_block", "BTC blocked"],
              ["tight_spread", "Spread ≤5 bps"],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              type="button"
              className={filter === id ? "chip active" : "chip"}
              onClick={() => setFilter(id)}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      <div className="table-wrap">
        <table className="radar-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Symbol</th>
              <th>Gap</th>
              <th>Score</th>
              <th>Δ</th>
              <th>Components</th>
              <th>Gates</th>
              <th>Bottleneck</th>
              <th>Decision</th>
              <th>Bot</th>
              <th>Reason</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr
                key={row.symbol}
                className={`radar-row tier-${(row.tier ?? "unknown").replace("+", "plus")} ${row.isCore ? "core" : ""}`}
              >
                <td>{row.rank}</td>
                <td>
                  <div className="sym-cell">
                    <strong>{(row.symbol ?? "").replace("USDT", "")}</strong>
                    {row.isCore && <span className="tag-core">core</span>}
                    {row.sideHint && (
                      <span className="tag-side">{row.sideHint}</span>
                    )}
                  </div>
                  <div className="sym-sub">
                    {fmtNum(row.price, 4)} · {(row.quoteVolume24h / 1e6).toFixed(0)}
                    M · {fmtNum(row.spreadBps, 1)} bps
                  </div>
                </td>
                <td className="gap-cell">
                  {row.gapToTrade > 0 ? (
                    <>+{row.gapToTrade.toFixed(2)}</>
                  ) : (
                    <span className="pos">ready</span>
                  )}
                </td>
                <td>{fmtScore(row.tradeScore)}</td>
                <td
                  className={
                    row.scoreDelta > 0.01
                      ? "pos"
                      : row.scoreDelta < -0.01
                        ? "neg"
                        : ""
                  }
                >
                  {row.scoreDelta >= 0 ? "+" : ""}
                  {row.scoreDelta.toFixed(2)}
                  <div className="sym-sub">
                    {row.priceDeltaPct >= 0 ? "+" : ""}
                    {row.priceDeltaPct.toFixed(2)}% px
                  </div>
                </td>
                <td>
                  <ComponentBars c={row.components} minScore={meta.minTradeScore} />
                </td>
                <td className="gates-cell" title="min · struct · flow · btc · spread">
                  {gateIcon(row.gates.minScore)}
                  {gateIcon(row.gates.structure)}
                  {gateIcon(row.gates.flow)}
                  {gateIcon(row.gates.btc)}
                  {gateIcon(row.gates.spread)}
                </td>
                <td>
                  <span className="weak-tag">{weakestLabel(row.weakestLink)}</span>
                  <div className="sym-sub">{row.btcRegime}</div>
                </td>
                <td className={`decision ${row.decision}`}>
                  {row.decision}
                  <div className="sym-sub">{row.tier}</div>
                </td>
                <td>
                  {row.willFire ? (
                    <span className="pos">will fire</span>
                  ) : (
                    <span className="muted">no</span>
                  )}
                </td>
                <td className="reason">{row.reason}</td>
              </tr>
            ))}
            {!rows.length && (
              <tr>
                <td colSpan={11} className="empty">
                  No symbols match this filter
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </section>
  );
}
