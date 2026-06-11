"use client";

import { fmtNum } from "@/lib/format";
import type { ProSignal } from "@/lib/types";

type Props = {
  rows: ProSignal[];
  floor: number;
  busy: string | null;
  onExecute: (symbol: string) => void;
};

function tierClass(tier: string) {
  switch (tier) {
    case "READY":
      return "tier-ready";
    case "BUILDING":
      return "tier-building";
    default:
      return "tier-below";
  }
}

export function UniverseScan({ rows, floor, busy, onExecute }: Props) {
  return (
    <section className="universe-section">
      <div className="section-head-row">
        <div>
          <span className="section-tag">Universe scan</span>
          <h2 className="section-title">
            Top <span className="mono">{rows.length}</span> by strength · floor{" "}
            <span className="mono">{floor}</span>
          </h2>
        </div>
      </div>

      <div className="universe-table-wrap">
        <table className="universe-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Symbol</th>
              <th>Str</th>
              <th>Tier</th>
              <th>Gap</th>
              <th>Playbook</th>
              <th>Side</th>
              <th>Flow</th>
              <th>Weak</th>
              <th>Vol 24h</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr
                key={r.symbol}
                className={`${tierClass(r.tier)} ${r.willFire ? "row-fire" : ""} ${r.isCore ? "row-core" : ""}`}
              >
                <td className="mono">{r.rank}</td>
                <td>
                  <strong>{r.symbol.replace("USDT", "")}</strong>
                  {r.isCore && <span className="core-dot">core</span>}
                  {r.playbookTriggered && <span className="trig-dot">▲</span>}
                </td>
                <td>
                  <span className="str-cell mono">{r.strength}</span>
                </td>
                <td>
                  <span className={`tier-badge ${tierClass(r.tier)}`}>{r.tier}</span>
                </td>
                <td className="mono">{r.gapToFloor > 0 ? `+${r.gapToFloor}` : "—"}</td>
                <td className="pb-cell">{r.playbook.replace(/_/g, " ")}</td>
                <td className={r.side === "LONG" ? "pos" : "neg"}>{r.side}</td>
                <td className="mono flow-cell">
                  {r.extra.takerFlow}/{r.extra.cvdState}
                </td>
                <td className="weak-cell">{r.weakestLink}</td>
                <td className="mono">{(r.quoteVolume24h / 1e6).toFixed(0)}M</td>
                <td>
                  {r.canExecute ? (
                    <button
                      type="button"
                      className="btn-exec"
                      disabled={busy !== null}
                      onClick={() => onExecute(r.symbol)}
                    >
                      {busy === r.symbol ? "…" : "Execute"}
                    </button>
                  ) : (
                    <span className="block-hint" title={r.blockReason}>
                      {r.blockReason?.replace(/_/g, " ") ?? "—"}
                    </span>
                  )}
                </td>
              </tr>
            ))}
            {!rows.length && (
              <tr>
                <td colSpan={11} className="empty">
                  Waiting for universe data…
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </section>
  );
}
