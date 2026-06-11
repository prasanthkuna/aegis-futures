"use client";

import { fmtNum } from "@/lib/format";
import type { SessionCockpit as SessionData, SignalsData } from "@/lib/types";

type Props = {
  session: SessionData | null;
  regime: SignalsData["regime"] | null;
  floor: number;
};

export function SessionStrip({ session, regime, floor }: Props) {
  if (!session) {
    return (
      <div className="session-strip skeleton-strip">
        <span className="mono">Syncing session…</span>
      </div>
    );
  }

  const tradePct = session.maxTradesPerDay
    ? Math.min(100, (session.tradesToday / session.maxTradesPerDay) * 100)
    : 0;

  return (
    <div className={`session-strip regime-${(regime?.label ?? "chop").toLowerCase()}`}>
      <div className="strip-block strip-session">
        <span className="strip-label">Session</span>
        <strong className="strip-value">{session.session.toUpperCase()}</strong>
      </div>

      <div className="strip-divider" />

      <div className="strip-block">
        <span className="strip-label">Regime</span>
        <strong className="strip-value">{session.regimeLabel || regime?.label || "—"}</strong>
        <span className="strip-hint">{regime?.summary ?? `${session.signalCount} signals`}</span>
      </div>

      <div className="strip-divider" />

      <div className="strip-block strip-floor">
        <span className="strip-label">Strength floor</span>
        <strong className="strip-value mono floor-num">{floor}</strong>
        <span className="strip-hint">
          {session.nextFloorDrop ? `↓ ${session.nextFloorDrop}` : "Locked"}
        </span>
      </div>

      <div className="strip-divider" />

      <div className="strip-block strip-trades">
        <span className="strip-label">Trades</span>
        <strong className="strip-value mono">
          {session.tradesToday}/{session.maxTradesPerDay}
        </strong>
        <div className="strip-bar">
          <div className="strip-bar-fill" style={{ width: `${tradePct}%` }} />
        </div>
        <span className="strip-hint">
          tgt {session.targetTradesPerDay} · min {session.minTradesPerDay}
        </span>
      </div>

      <div className="strip-divider" />

      <div className="strip-block">
        <span className="strip-label">BTC 5m</span>
        <strong
          className={`strip-value mono ${session.btcChange5mPct >= 0 ? "pos" : "neg"}`}
        >
          {session.btcChange5mPct >= 0 ? "+" : ""}
          {fmtNum(session.btcChange5mPct, 2)}%
        </strong>
      </div>

      <div className="strip-playbooks">
        {(session.activePlaybooks ?? []).map((pb) => (
          <span key={pb} className={`pb-tag pb-${pb.toLowerCase()}`}>
            {pb.replace(/_/g, " ")}
          </span>
        ))}
      </div>
    </div>
  );
}
