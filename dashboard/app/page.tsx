"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";

type Summary = {
  botStatus: string;
  activeCapitalUsd: number;
  accountBalance: number;
  availableMargin: number;
  todayPnl: number;
  weeklyPnl: number;
  netPnlAfterFees: number;
  feesPaid: number;
  killSwitchActive: boolean;
  tradingEnabled: boolean;
  testnet: boolean;
  hasOpenPosition: boolean;
};

type RadarItem = {
  symbol: string;
  price: number;
  quoteVolume24h: number;
  spreadBps: number;
  tradeScore: number;
  decision: string;
  reason: string;
};

export default function Home() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [radar, setRadar] = useState<RadarItem[]>([]);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [s, r] = await Promise.all([
        api<Summary>("/dashboard/summary"),
        api<{ items: RadarItem[] }>("/radar"),
      ]);
      setSummary(s);
      setRadar(r.items ?? []);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "load failed");
    }
  }, []);

  useEffect(() => {
    load();
    const id = setInterval(load, 5000);
    return () => clearInterval(id);
  }, [load]);

  async function action(path: string) {
    await api(path, { method: "POST" });
    await load();
  }

  return (
    <main>
      <header>
        <div>
          <h1>Aegis Futures</h1>
          <p style={{ color: "var(--muted)", margin: "4px 0 0" }}>
            Mainnet-first · no auth · rolling 24h volume
          </p>
        </div>
        <div className="actions">
          <button className="start" onClick={() => action("/bot/start")}>
            Start
          </button>
          <button className="pause" onClick={() => action("/bot/pause")}>
            Pause
          </button>
          <button className="kill" onClick={() => action("/bot/kill")}>
            Kill
          </button>
        </div>
      </header>

      {error && <p style={{ color: "var(--danger)" }}>{error}</p>}

      {summary && (
        <section className="grid">
          <div className="card">
            <label>Bot status</label>
            <strong>{summary.botStatus}</strong>
          </div>
          <div className="card">
            <label>Active capital</label>
            <strong>${summary.activeCapitalUsd.toFixed(0)}</strong>
          </div>
          <div className="card">
            <label>Balance</label>
            <strong>${summary.accountBalance.toFixed(2)}</strong>
          </div>
          <div className="card">
            <label>Today PnL</label>
            <strong>${summary.todayPnl.toFixed(2)}</strong>
          </div>
          <div className="card">
            <label>Net after fees</label>
            <strong>${summary.netPnlAfterFees.toFixed(2)}</strong>
          </div>
          <div className="card">
            <label>Trading</label>
            <strong>{summary.tradingEnabled ? "ON" : "OFF"}</strong>
          </div>
          <div className="card">
            <label>Network</label>
            <strong>{summary.testnet ? "testnet" : "mainnet"}</strong>
          </div>
          <div className="card">
            <label>Kill switch</label>
            <strong>{summary.killSwitchActive ? "ACTIVE" : "off"}</strong>
          </div>
        </section>
      )}

      <h2 style={{ fontSize: "1rem", marginBottom: 8 }}>Setup radar</h2>
      <table>
        <thead>
          <tr>
            <th>Symbol</th>
            <th>Price</th>
            <th>Rolling 24h vol</th>
            <th>Spread bps</th>
            <th>Score</th>
            <th>Decision</th>
            <th>Reason</th>
          </tr>
        </thead>
        <tbody>
          {radar.map((row) => (
            <tr key={row.symbol}>
              <td>{row.symbol}</td>
              <td>{row.price.toFixed(4)}</td>
              <td>{(row.quoteVolume24h / 1e6).toFixed(1)}M</td>
              <td>{row.spreadBps.toFixed(1)}</td>
              <td>{row.tradeScore.toFixed(2)}</td>
              <td className={row.decision === "trade" ? "trade" : "skip"}>
                {row.decision}
              </td>
              <td>{row.reason}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </main>
  );
}
