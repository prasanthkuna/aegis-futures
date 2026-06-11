"use client";

import { fmtDuration, fmtNum, fmtUsd, pnlClass } from "@/lib/format";
import type { PositionLiveData } from "@/lib/types";

type Props = { data: PositionLiveData | null; isCore?: boolean };

function StrengthRing({ value, max = 100 }: { value: number; max?: number }) {
  const pct = Math.min(1, value / max);
  const r = 42;
  const c = 2 * Math.PI * r;
  const offset = c * (1 - pct);
  return (
    <svg className="r-ring" viewBox="0 0 100 100" aria-hidden>
      <circle className="r-ring-bg" cx="50" cy="50" r={r} />
      <circle
        className="r-ring-fill"
        cx="50"
        cy="50"
        r={r}
        strokeDasharray={c}
        strokeDashoffset={offset}
      />
    </svg>
  );
}

export function PositionCommander({ data, isCore }: Props) {
  if (!data?.hasPosition) {
    return (
      <section className="commander-panel flat-state">
        <div className="flat-scan" aria-hidden />
        <span className="section-tag">Position</span>
        <div className="flat-inner">
          <span className="flat-icon" aria-hidden>
            ◎
          </span>
          <h2>Flat</h2>
          <p>
            {isCore
              ? "Watching BTC · ETH · SOL for 1h playbook triggers"
              : "Scanning universe for signals above floor"}
          </p>
        </div>
      </section>
    );
  }

  const p = data.position;
  const phase = p.exitPhase || "PROTECTED";
  const rPct = Math.min(100, Math.max(0, (p.rMultiple + 1) * 33));
  const label = p.paper ? "Paper Position" : "Live Position";

  return (
    <section className={`commander-panel in-position${p.paper ? " paper-position" : ""}`}>
      <div className="commander-top">
        <span className="section-tag">{label}</span>
        <span className={`phase-tag phase-${phase.toLowerCase()}`}>{phase}</span>
      </div>

      <div className="commander-body">
        <div className="commander-left">
          <div className="sym-row">
            <h2>{p.symbol.replace("USDT", "")}</h2>
            <span className={`side-tag ${p.side === "LONG" ? "long" : "short"}`}>
              {p.side}
            </span>
            <span className="lev-tag mono">{p.leverage}×</span>
          </div>

          <div className={`pnl-display ${pnlClass(p.unrealizedPnl)}`}>
            <span className="pnl-label">Unrealized</span>
            <span className="pnl-num mono">{fmtUsd(p.unrealizedPnl)}</span>
          </div>

          <div className="commander-stats mono">
            <span>Entry {fmtNum(p.entryPrice, 4)}</span>
            <span>Mark {fmtNum(p.markPrice, 4)}</span>
            <span>Qty {fmtNum(p.remainingQty, 4)}</span>
            <span>{fmtDuration(p.holdSec)}</span>
          </div>
        </div>

        <div className="commander-r">
          <div className="r-gauge">
            <StrengthRing value={rPct} />
            <div className="r-center mono">
              <strong>{fmtNum(p.rMultiple, 2)}</strong>
              <span>R</span>
            </div>
          </div>
          <span className="r-peak mono">peak {fmtNum(p.peakR, 2)}R</span>
        </div>
      </div>

      <div className="commander-footer">
        <div className="stop-row mono">
          <span>
            SL <strong>{fmtNum(p.stopPrice, 4)}</strong>
          </span>
          <span>
            TP <strong>{fmtNum(p.takeProfitPrice, 4)}</strong>
          </span>
          {p.trailPrice ? (
            <span>
              Trail <strong>{fmtNum(p.trailPrice, 4)}</strong>
            </span>
          ) : null}
        </div>

        <div className="meta-row">
          <span className="pb-tag pb-generic">{p.playbook.replace(/_/g, " ")}</span>
          <span className="mono">str {p.strengthAtEntry}</span>
          <span className="mono">partial {Math.round(p.partialPctDone * 100)}%</span>
          <span className={p.guardianStatus === "active" ? "guard-ok" : "guard-bad"}>
            {p.guardianStatus}
          </span>
        </div>

        <div className="rules-row">
          {(p.rulesArmed ?? []).map((r) => (
            <span key={r} className="rule-tag">
              {r.replace(/_/g, " ")}
            </span>
          ))}
        </div>
      </div>
    </section>
  );
}
