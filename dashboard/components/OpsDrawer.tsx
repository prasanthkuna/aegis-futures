"use client";

import { fmtNum, fmtTime, fmtUsd, pnlClass } from "@/lib/format";
import type { BotConfig, ClosedTrade, RiskEvent } from "@/lib/types";

type Props = {
  open: boolean;
  onClose: () => void;
  tab: "trades" | "risk" | "config";
  onTab: (t: "trades" | "risk" | "config") => void;
  trades: ClosedTrade[];
  events: RiskEvent[];
  config: BotConfig | null;
  configDraft: {
    active: string;
    risk: string;
    maxTrades: string;
    dailyStop: string;
    weeklyStop: string;
    maxLev: string;
  };
  onDraft: (patch: Partial<Props["configDraft"]>) => void;
  onSave: () => void;
  busy: boolean;
};

export function OpsDrawer({
  open,
  onClose,
  tab,
  onTab,
  trades,
  events,
  config,
  configDraft,
  onDraft,
  onSave,
  busy,
}: Props) {
  if (!open) return null;

  return (
    <div className="drawer-backdrop" onClick={onClose} role="presentation">
      <aside
        className="ops-drawer"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label="Operations"
      >
        <div className="drawer-head">
          <h2>Operations</h2>
          <button type="button" className="btn-ghost" onClick={onClose}>
            ✕
          </button>
        </div>

        <div className="drawer-tabs">
          {(["trades", "risk", "config"] as const).map((t) => (
            <button
              key={t}
              type="button"
              className={tab === t ? "dtab active" : "dtab"}
              onClick={() => onTab(t)}
            >
              {t}
            </button>
          ))}
        </div>

        <div className="drawer-body">
          {tab === "trades" && (
            <div className="drawer-table-wrap">
              <table className="drawer-table">
                <thead>
                  <tr>
                    <th>Sym</th>
                    <th>Side</th>
                    <th>Net</th>
                    <th>R</th>
                    <th>Exit</th>
                  </tr>
                </thead>
                <tbody>
                  {trades.map((t) => (
                    <tr key={t.id}>
                      <td>{t.symbol.replace("USDT", "")}</td>
                      <td>{t.side}</td>
                      <td className={pnlClass(t.netPnl)}>{fmtUsd(t.netPnl)}</td>
                      <td className="mono">{fmtNum(t.rMultiple, 2)}R</td>
                      <td className="drawer-reason">{t.exitReason || "—"}</td>
                    </tr>
                  ))}
                  {!trades.length && (
                    <tr>
                      <td colSpan={5} className="empty">
                        No closed trades
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          )}

          {tab === "risk" && (
            <div className="drawer-table-wrap">
              <table className="drawer-table">
                <thead>
                  <tr>
                    <th>Time</th>
                    <th>Sev</th>
                    <th>Type</th>
                    <th>Msg</th>
                  </tr>
                </thead>
                <tbody>
                  {events.map((e) => (
                    <tr key={e.id}>
                      <td className="mono">{fmtTime(e.timestamp)}</td>
                      <td className={`sev ${e.severity}`}>{e.severity}</td>
                      <td>{e.type}</td>
                      <td className="drawer-reason">{e.message}</td>
                    </tr>
                  ))}
                  {!events.length && (
                    <tr>
                      <td colSpan={4} className="empty">
                        No risk events
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          )}

          {tab === "config" && config && (
            <div className="drawer-config">
              <dl className="config-readout">
                <dt>Account</dt>
                <dd className="mono">{fmtUsd(config.accountCapitalUsd, 0)}</dd>
                <dt>Max lev</dt>
                <dd className="mono">{config.maxLeverage}×</dd>
                <dt>Max trades</dt>
                <dd className="mono">{config.maxTradesPerDay}</dd>
              </dl>
              <div className="config-fields">
                <label>
                  Active capital
                  <input
                    value={configDraft.active}
                    onChange={(e) => onDraft({ active: e.target.value })}
                  />
                </label>
                <label>
                  Risk / trade
                  <input
                    value={configDraft.risk}
                    onChange={(e) => onDraft({ risk: e.target.value })}
                  />
                </label>
                <label>
                  Max trades / day
                  <input
                    value={configDraft.maxTrades}
                    onChange={(e) => onDraft({ maxTrades: e.target.value })}
                  />
                </label>
                <label>
                  Daily stop
                  <input
                    value={configDraft.dailyStop}
                    onChange={(e) => onDraft({ dailyStop: e.target.value })}
                  />
                </label>
                <label>
                  Weekly stop
                  <input
                    value={configDraft.weeklyStop}
                    onChange={(e) => onDraft({ weeklyStop: e.target.value })}
                  />
                </label>
                <label>
                  Max leverage
                  <input
                    value={configDraft.maxLev}
                    onChange={(e) => onDraft({ maxLev: e.target.value })}
                  />
                </label>
                <button
                  type="button"
                  className="btn-go"
                  disabled={busy}
                  onClick={onSave}
                >
                  {busy ? "Saving…" : "Save"}
                </button>
              </div>
            </div>
          )}
        </div>
      </aside>
    </div>
  );
}
