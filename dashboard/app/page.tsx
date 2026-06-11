"use client";

import { useCallback, useEffect, useState } from "react";
import { MetricsRail } from "@/components/MetricsRail";
import { OpsDrawer } from "@/components/OpsDrawer";
import { PositionCommander } from "@/components/PositionCommander";
import { SessionStrip } from "@/components/SessionStrip";
import { SignalBoard } from "@/components/SignalBoard";
import { TopBar } from "@/components/TopBar";
import { api } from "@/lib/api";
import type {
  BotConfig,
  ClosedTrade,
  DashboardData,
  PositionLiveData,
  RiskEvent,
  SessionCockpit,
  SignalsData,
  StrategyTruth,
  Summary,
} from "@/lib/types";

const POLL_MS = 5000;
const POLL_SEC = POLL_MS / 1000;

const emptyTruth: StrategyTruth = {
  winRate: 0,
  avgWin: 0,
  avgLoss: 0,
  profitFactor: 0,
  expectancyAfterFees: 0,
  maxDrawdown: 0,
  closedTradeCount: 0,
};

const emptySignals: SignalsData = {
  signals: [],
  session: {
    session: "—",
    floor: 55,
    tradesToday: 0,
    minTradesPerDay: 2,
    maxTradesPerDay: 6,
    targetTradesPerDay: 4,
    activePlaybooks: [],
    btcChange5mPct: 0,
    armed: false,
    tradingEnabled: false,
    regimeLabel: "—",
    signalCount: 0,
  },
  floor: 55,
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
    signalsRes,
    sessionRes,
    positionLiveRes,
    tradesRes,
    truth,
    eventsRes,
    config,
    status,
  ] = await Promise.all([
    get("/dashboard/summary", null as unknown as Summary),
    get("/signals", emptySignals),
    get("/signals/session", null as unknown as SessionCockpit),
    get("/position/live", {
      hasPosition: false,
      position: {} as PositionLiveData["position"],
    }),
    get("/trades/closed", { trades: [] as ClosedTrade[] }),
    get("/dashboard/strategy-truth", emptyTruth),
    get("/risk-events", { events: [] as RiskEvent[] }),
    get("/config/current", null as unknown as BotConfig),
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

  return {
    data: {
      summary,
      signals: signalsRes ?? null,
      session: sessionRes ?? signalsRes?.session ?? null,
      positionLive: positionLiveRes ?? null,
      trades: tradesRes.trades ?? [],
      truth,
      events: eventsRes.events ?? [],
      config:
        config ??
        ({
          accountCapitalUsd: 0,
          activeCapitalUsd: 0,
          maxLeverage: 3,
          riskPerTradeUsd: 0,
          maxOpenPositions: 1,
          maxTradesPerDay: 6,
          dailyHardStopUsd: 0,
          weeklyHardStopUsd: 0,
          minTradeScore: 0,
        } satisfies BotConfig),
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
  const [opsOpen, setOpsOpen] = useState(false);
  const [opsTab, setOpsTab] = useState<"trades" | "risk" | "config">("trades");
  const [loaded, setLoaded] = useState(false);
  const [configDraft, setConfigDraft] = useState({
    active: "",
    risk: "",
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
        maxTrades: String(d.config.maxTradesPerDay),
        dailyStop: String(d.config.dailyHardStopUsd),
        weeklyStop: String(d.config.weeklyHardStopUsd),
        maxLev: String(d.config.maxLeverage),
      });
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "load failed");
    } finally {
      setLoaded(true);
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
    const maxTrades = parseInt(configDraft.maxTrades, 10);
    const dailyStop = parseFloat(configDraft.dailyStop);
    const weeklyStop = parseFloat(configDraft.weeklyStop);
    const maxLev = parseInt(configDraft.maxLev, 10);
    if (Number.isNaN(active) || Number.isNaN(risk)) return;
    setBusy("config");
    try {
      await api("/config/update", {
        method: "POST",
        body: JSON.stringify({
          activeCapitalUsd: active,
          riskPerTradeUsd: risk,
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

  const signalWarnings = warnings.filter(
    (w) => w.startsWith("/signals") || w.startsWith("/position")
  );

  return (
    <div className={`terminal ${loaded ? "terminal-loaded" : ""}`}>
      <div className="terminal-bg" aria-hidden>
        <div className="grid-overlay" />
        <div className="scanlines" />
        <div className="glow-orb glow-orb-a" />
        <div className="glow-orb glow-orb-b" />
      </div>

      <TopBar
        status={data?.status ?? null}
        summary={data?.summary ?? null}
        pollSec={POLL_SEC}
        busy={busy}
        onStart={() => botAction("/bot/start")}
        onPause={() => botAction("/bot/pause")}
        onKill={() => botAction("/bot/kill")}
        onOps={() => setOpsOpen(true)}
      />

      {error && (
        <div className="alert alert-error" role="alert">
          {error}
        </div>
      )}
      {signalWarnings.length > 0 && !error && (
        <div className="alert alert-warn" role="status">
          Deploy backend to enable {signalWarnings.join(", ")}.
        </div>
      )}
      {data?.status && !data.status.tradingEnabled && !error && (
        <div className="alert alert-info" role="status">
          Trading secret off — signals display but entries won&apos;t fire until{" "}
          <code>AegisTradingEnabled=true</code>
        </div>
      )}

      <SessionStrip
        session={data?.session ?? null}
        regime={data?.signals?.regime ?? null}
        floor={data?.signals?.floor ?? 55}
      />

      <div className="hero-grid">
        <PositionCommander data={data?.positionLive ?? null} />
        <MetricsRail summary={data?.summary ?? null} truth={data?.truth ?? null} />
      </div>

      <SignalBoard data={data?.signals ?? null} />

      <OpsDrawer
        open={opsOpen}
        onClose={() => setOpsOpen(false)}
        tab={opsTab}
        onTab={setOpsTab}
        trades={data?.trades ?? []}
        events={data?.events ?? []}
        config={data?.config ?? null}
        configDraft={configDraft}
        onDraft={(p) => setConfigDraft((d) => ({ ...d, ...p }))}
        onSave={saveConfig}
        busy={busy === "config"}
      />
    </div>
  );
}
