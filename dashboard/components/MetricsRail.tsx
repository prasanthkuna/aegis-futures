"use client";

import { fmtPct, fmtUsd, pnlClass } from "@/lib/format";
import type { StrategyTruth, Summary } from "@/lib/types";

type Props = {
  summary: Summary | null;
  truth: StrategyTruth | null;
  isPaper?: boolean;
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

export function MetricsRail({ summary, truth, isPaper }: Props) {
  const s = summary;
  const capital = s?.activeCapitalUsd ?? 0;
  const weekNet = s?.netPnlAfterFees ?? 0;
  const roiPct = capital > 0 ? (weekNet / capital) * 100 : 0;

  return (
    <aside className="metrics-rail">
      <div className="metrics-head">
        <span className="section-tag">{isPaper ? "Paper PnL" : "PnL & Risk"}</span>
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
        <Metric
          label={isPaper ? "Sim equity" : "Balance"}
          value={fmtUsd(s?.accountBalance ?? 0)}
        />
        {!isPaper && <Metric label="Margin" value={fmtUsd(s?.availableMargin ?? 0)} />}
        <Metric label="Capital" value={fmtUsd(capital, 0)} />
        {isPaper && (
          <Metric
            label="ROI"
            value={fmtPct(roiPct)}
            className={pnlClass(weekNet)}
            sub="on active capital"
          />
        )}
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
          value={isPaper ? "PAPER" : s?.tradingEnabled ? "ON" : "OFF"}
          className={isPaper || s?.tradingEnabled ? "pos" : ""}
        />
      </div>
    </aside>
  );
}
