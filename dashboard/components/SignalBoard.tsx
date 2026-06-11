"use client";

import type { ProSignal } from "@/lib/types";

type Props = {
  signals: ProSignal[];
  floor: number;
  busy: string | null;
  onExecute?: (symbol: string) => void;
  isCore?: boolean;
};

function StrengthRing({ strength }: { strength: number }) {
  const pct = strength / 100;
  const r = 36;
  const c = 2 * Math.PI * r;
  return (
    <div className="sig-ring-wrap ready">
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

function SignalCard({
  sig,
  busy,
  onExecute,
  isCore,
}: {
  sig: ProSignal;
  busy: string | null;
  onExecute?: (s: string) => void;
  isCore?: boolean;
}) {
  const sym = sig.symbol.replace("USDT", "");
  const pbClass = `pb-${sig.playbook.toLowerCase()}`;

  return (
    <article
      className={`signal-card ${sig.willFire ? "will-fire" : ""} ${sig.isCore ? "is-core" : ""}`}
      style={{ animationDelay: `${Math.min(sig.rank, 12) * 40}ms` }}
    >
      {sig.willFire && <div className="fire-edge" aria-hidden />}

      <div className="sig-card-top">
        <StrengthRing strength={sig.strength} />
        <div className="sig-card-meta">
          <div className="sig-sym-row">
            <h3>{sym}</h3>
            {sig.isCore && <span className="core-dot">core</span>}
            <span className={`side-tag ${sig.side === "LONG" ? "long" : "short"}`}>
              {sig.side}
            </span>
            <span className="tier-badge tier-ready">READY</span>
          </div>
          <span className={`pb-tag ${pbClass}`}>{sig.playbook.replace(/_/g, " ")}</span>
        </div>
      </div>

      <div className="sig-card-actions">
        {sig.canExecute && onExecute ? (
          <button
            type="button"
            className="btn-exec wide"
            disabled={busy !== null}
            onClick={() => onExecute(sig.symbol)}
          >
            {busy === sig.symbol ? "Placing…" : "Execute"}
          </button>
        ) : (
          <span className="block-hint">{sig.blockReason?.replace(/_/g, " ") ?? "blocked"}</span>
        )}
        {sig.willFire && (
          <span className="fire-badge">
            {isCore ? "AUTO ON 1H CLOSE" : "AUTO WILL FIRE"}
          </span>
        )}
      </div>
    </article>
  );
}

export function SignalBoard({ signals, floor, busy, onExecute, isCore }: Props) {
  return (
    <section className="signal-section">
      <div className="signal-section-head">
        <div>
          <span className="section-tag">
            {isCore ? "Core playbooks" : "Ready signals"}
          </span>
          <h2 className="signal-title">
            {isCore ? (
              <>
                {signals.length} triggered · auto paper entry on 1h close
              </>
            ) : (
              <>
                {signals.length} above floor <span className="mono">{floor}</span>
              </>
            )}
          </h2>
        </div>
      </div>

      <div className="signal-grid">
        {signals.map((sig) => (
          <SignalCard key={sig.symbol} sig={sig} busy={busy} onExecute={onExecute} isCore={isCore} />
        ))}
        {!signals.length && (
          <div className="signal-empty">
            <p>
              {isCore
                ? "No 1h playbook triggers right now"
                : `No symbols above strength floor ${floor}`}
            </p>
            <span className="mono">
              {isCore
                ? "BTC S4 · ETH S11 · SOL S4 — entries fire automatically in paper mode"
                : "Check universe scan below — setups may be building"}
            </span>
          </div>
        )}
      </div>
    </section>
  );
}
