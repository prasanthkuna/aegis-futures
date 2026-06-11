"use client";

import { fmtPct, fmtUsd, pnlClass } from "@/lib/format";
import type { StrategyTruth, Summary } from "@/lib/types";

type Props = {
  summary: Summary | null;
  truth: StrategyTruth | null;
};

function Metric({
  label,
  value,
  className = "",
  sub,
}: {
  label: string;
  value: string;
  className?: string;
  sub?: string;
}) {
  return (
    <div className={`metric-cell ${className}`}>
      <span className="metric-label">{label}</span>
      <strong className={`metric-value mono ${className}`}>{value}</strong>
      {sub && <span className="metric-sub">{sub}</span>}
    </div>
  );
}

export function MetricsRail({ summary, truth }: Props) {
  const s = summary;
  return (
    <aside className="metrics-rail">
      <div className="metrics-head">
        <span className="section-tag">PnL & Risk</span>
        {s?.killSwitchActive && <span className="kill-flag">KILL ACTIVE</span>}
      </div>

      <div className="metrics-hero">
        <Metric
          label="Today"
          value={fmtUsd(s?.todayPnl ?? 0)}
          className={pnlClass(s?.todayPnl ?? 0)}
        />
        <Metric
          label="Open"
          value={fmtUsd(s?.openPnl ?? 0)}
          className={pnlClass(s?.openPnl ?? 0)}
        />
        <Metric
          label="Week net"
          value={fmtUsd(s?.netPnlAfterFees ?? 0)}
          className={pnlClass(s?.netPnlAfterFees ?? 0)}
        />
      </div>

      <div className="metrics-grid">
        <Metric label="Balance" value={fmtUsd(s?.accountBalance ?? 0)} />
        <Metric label="Margin" value={fmtUsd(s?.availableMargin ?? 0)} />
        <Metric label="Capital" value={fmtUsd(s?.activeCapitalUsd ?? 0, 0)} />
        <Metric label="Drawdown" value={fmtUsd(s?.currentDrawdown ?? 0)} className="neg" />
        <Metric label="Fees" value={fmtUsd(s?.feesPaid ?? 0)} />
        <Metric
          label="Expectancy"
          value={fmtUsd(truth?.expectancyAfterFees ?? 0, 3)}
          className={pnlClass(truth?.expectancyAfterFees ?? 0)}
        />
        <Metric label="Win rate" value={fmtPct(truth?.winRate ?? 0)} />
        <Metric label="PF" value={(truth?.profitFactor ?? 0).toFixed(2)} />
        <Metric label="Closed" value={String(truth?.closedTradeCount ?? 0)} />
        <Metric
          label="Trading"
          value={s?.tradingEnabled ? "ON" : "OFF"}
          className={s?.tradingEnabled ? "pos" : ""}
        />
      </div>
    </aside>
  );
}
