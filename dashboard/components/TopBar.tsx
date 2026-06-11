"use client";

import type { BotStatus, Summary } from "@/lib/types";

type Props = {
  status: BotStatus | null;
  summary: Summary | null;
  pollSec: number;
  busy: string | null;
  onStart: () => void;
  onPause: () => void;
  onKill: () => void;
  onOps: () => void;
};

export function TopBar({
  status,
  summary,
  pollSec,
  busy,
  onStart,
  onPause,
  onKill,
  onOps,
}: Props) {
  const armed = status?.armed;
  const state = status?.state ?? "—";
  const isCore = status?.tradingMode === "core_swing";
  const isPaper = status?.paperMode;

  return (
    <header className="topbar">
      <div className="brand">
        <div className="brand-mark" aria-hidden />
        <div>
          <h1>AEGIS</h1>
          <span className="brand-sub">
            {isCore ? "Core Swing Command" : "Futures Command"}
          </span>
        </div>
      </div>

      <div className="status-rail">
        <span className={`pill state-${state.toLowerCase()}`}>{state}</span>
        {isPaper ? (
          <span className="pill pill-paper">
            <span className="pulse" aria-hidden />
            PAPER
          </span>
        ) : status?.tradingEnabled ? (
          <span className="pill pill-live">
            <span className="pulse" aria-hidden />
            LIVE
          </span>
        ) : (
          <span className={`pill ${armed ? "pill-live" : "pill-dim"}`}>
            <span className="pulse" aria-hidden />
            {armed ? "ARMED" : "STANDBY"}
          </span>
        )}
        <span className={`pill ${isCore ? "pill-live" : "pill-dim"}`}>
          {isCore
            ? `CORE 1H${status?.aggressive ? " AGG" : ""}`
            : "ALT SCAN"}
        </span>
        <span className="pill pill-dim">
          {status?.universeSize ?? 0} sym
        </span>
        <span className="pill pill-dim">
          {summary?.testnet ? "TESTNET" : "MAINNET"}
        </span>
        <span className="pill pill-dim mono">{pollSec}s</span>
      </div>

      <div className="topbar-actions">
        <button type="button" className="btn-ghost" onClick={onOps}>
          Ops
        </button>
        <button
          type="button"
          className="btn-go"
          disabled={!!busy}
          onClick={onStart}
        >
          {busy === "/bot/start" ? "…" : "Start"}
        </button>
        <button
          type="button"
          className="btn-warn"
          disabled={!!busy}
          onClick={onPause}
        >
          {busy === "/bot/pause" ? "…" : "Pause"}
        </button>
        <button
          type="button"
          className="btn-kill"
          disabled={!!busy}
          onClick={onKill}
        >
          {busy === "/bot/kill" ? "…" : "Kill"}
        </button>
      </div>
    </header>
  );
}
