"use client";

import type { EngineHeartbeat as HB, SignalsData } from "@/lib/types";

type Props = {
  heartbeat: HB | null;
  narrative: string;
  regime: SignalsData["regime"] | null;
};

export function EngineHeartbeat({ heartbeat, narrative, regime }: Props) {
  const hb = heartbeat;
  return (
    <div className="heartbeat-bar">
      <div className="hb-pulse" aria-hidden />
      <div className="hb-grid">
        <div className="hb-cell">
          <span className="hb-label">Engine</span>
          <strong className={`mono ${hb?.marketDataHealthy ? "pos" : "neg"}`}>
            {hb?.botState ?? "—"}
          </strong>
        </div>
        <div className="hb-cell">
          <span className="hb-label">Universe</span>
          <strong className="mono">{hb?.symbolsScanned ?? 0}</strong>
        </div>
        <div className="hb-cell">
          <span className="hb-label">Above floor</span>
          <strong className="mono pos">{hb?.aboveFloor ?? 0}</strong>
        </div>
        <div className="hb-cell">
          <span className="hb-label">Near miss</span>
          <strong className="mono">{hb?.nearMissCount ?? 0}</strong>
        </div>
        <div className="hb-cell">
          <span className="hb-label">Will fire</span>
          <strong className={`mono ${(hb?.willFireCount ?? 0) > 0 ? "pos" : ""}`}>
            {hb?.willFireCount ?? 0}
          </strong>
        </div>
        <div className="hb-cell">
          <span className="hb-label">Peak / median</span>
          <strong className="mono">
            {hb?.maxStrength ?? 0}/{hb?.medianStrength ?? 0}
          </strong>
        </div>
        <div className="hb-cell">
          <span className="hb-label">Flat CVD</span>
          <strong className="mono">{hb?.flatCvdCount ?? 0}</strong>
        </div>
      </div>
      <p className="hb-narrative">
        <span className={`regime-pill regime-${(regime?.label ?? "chop").toLowerCase()}`}>
          {regime?.label ?? "—"}
        </span>
        {narrative || regime?.summary || "Scanning…"}
      </p>
    </div>
  );
}
