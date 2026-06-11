"use client";

import { useMemo, useState } from "react";
import { fmtNum } from "@/lib/format";
import type { ProSignal, SignalsData } from "@/lib/types";

type Props = { data: SignalsData | null };

const COMP_KEYS = [
  { key: "volume", label: "VOL", w: "vol" },
  { key: "cvd", label: "CVD", w: "cvd" },
  { key: "structure", label: "STR", w: "str" },
  { key: "context", label: "CTX", w: "ctx" },
  { key: "depth", label: "DEP", w: "dep" },
  { key: "session", label: "SES", w: "ses" },
] as const;

function StrengthRing({ strength, floor }: { strength: number; floor: number }) {
  const hot = strength >= floor + 15;
  const ready = strength >= floor;
  const pct = strength / 100;
  const r = 36;
  const c = 2 * Math.PI * r;
  return (
    <div
      className={`sig-ring-wrap ${hot ? "hot" : ready ? "ready" : "warm"}`}
    >
      <svg viewBox="0 0 80 80" className="sig-ring" aria-hidden>
        <circle className="sig-ring-bg" cx="40" cy="40" r={r} />
        <circle
          className="sig-ring-fill"
          cx="40"
          cy="40"
          r={r}
          strokeDasharray={c}
          strokeDashoffset={c * (1 - pct)}
        />
      </svg>
      <span className="sig-ring-num mono">{strength}</span>
    </div>
  );
}

function SignalCard({ sig, floor }: { sig: ProSignal; floor: number }) {
  const sym = sig.symbol.replace("USDT", "");
  const pbClass = `pb-${sig.playbook.toLowerCase()}`;

  return (
    <article
      className={`signal-card ${sig.willFire ? "will-fire" : ""} ${sig.isCore ? "is-core" : ""}`}
      style={{ animationDelay: `${Math.min(sig.rank, 12) * 40}ms` }}
    >
      {sig.willFire && <div className="fire-edge" aria-hidden />}

      <div className="sig-card-top">
        <StrengthRing strength={sig.strength} floor={floor} />
        <div className="sig-card-meta">
          <div className="sig-sym-row">
            <h3>{sym}</h3>
            {sig.isCore && <span className="core-dot">core</span>}
            <span className={`side-tag ${sig.side === "LONG" ? "long" : "short"}`}>
              {sig.side}
            </span>
          </div>
          <span className={`pb-tag ${pbClass}`}>
            {sig.playbook.replace(/_/g, " ")}
          </span>
          <span className="sig-price mono">
            {fmtNum(sig.price, 4)} · {(sig.quoteVolume24h / 1e6).toFixed(0)}M ·{" "}
            {fmtNum(sig.spreadBps, 1)}bps
          </span>
        </div>
        <span className="sig-rank mono">#{sig.rank}</span>
      </div>

      <div className="sig-components">
        {COMP_KEYS.map(({ key, label }) => {
          const v = sig.components[key];
          return (
            <div key={key} className="sig-comp-row">
              <span className="sig-comp-lbl">{label}</span>
              <div className="sig-comp-track">
                <div
                  className="sig-comp-fill"
                  style={{ width: `${Math.round(v * 100)}%` }}
                />
              </div>
              <span className="sig-comp-val mono">{(v * 100).toFixed(0)}</span>
            </div>
          );
        })}
      </div>

      <div className="sig-card-foot">
        <span className="flow-tag mono">
          {sig.extra.takerFlow}/{sig.extra.cvdState}
        </span>
        <span className="mono vwap-tag">
          VWAP {sig.extra.vwapDevPct >= 0 ? "+" : ""}
          {sig.extra.vwapDevPct.toFixed(2)}%
        </span>
        <span className="btc-tag">{sig.extra.btcRegime}</span>
        {sig.willFire ? (
          <span className="fire-badge">WILL FIRE</span>
        ) : (
          <span className="standby-badge">standby</span>
        )}
      </div>
    </article>
  );
}

export function SignalBoard({ data }: Props) {
  const [filter, setFilter] = useState<"all" | "fire" | "core">("all");
  const floor = data?.floor ?? 55;

  const rows = useMemo(() => {
    let list = data?.signals ?? [];
    if (filter === "fire") list = list.filter((s) => s.willFire);
    if (filter === "core") list = list.filter((s) => s.isCore);
    return list;
  }, [data?.signals, filter]);

  return (
    <section className="signal-section">
      <div className="signal-section-head">
        <div>
          <span className="section-tag">Signal Board</span>
          <h2 className="signal-title">
            {rows.length} ranked · floor <span className="mono">{floor}</span>
          </h2>
        </div>
        <div className="signal-filters">
          {(
            [
              ["all", "All"],
              ["fire", "Will fire"],
              ["core", "Core"],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              type="button"
              className={filter === id ? "filt active" : "filt"}
              onClick={() => setFilter(id)}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      <div className="signal-grid">
        {rows.map((sig) => (
          <SignalCard key={sig.symbol} sig={sig} floor={floor} />
        ))}
        {!rows.length && (
          <div className="signal-empty">
            <p>No signals match this filter</p>
            <span className="mono">Waiting for engine scan…</span>
          </div>
        )}
      </div>
    </section>
  );
}
