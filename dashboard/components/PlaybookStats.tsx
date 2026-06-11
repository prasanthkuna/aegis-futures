"use client";

import { fmtNum, fmtPct, fmtUsd, pnlClass } from "@/lib/format";
import type { PlaybookStat } from "@/lib/types";

type Props = { stats: PlaybookStat[] };

export function PlaybookStats({ stats }: Props) {
  if (!stats.length) {
    return (
      <section className="pbstats-section">
        <span className="section-tag">Playbook stats</span>
        <p className="pbstats-empty">No closed trades yet — stats appear after first exits</p>
      </section>
    );
  }

  return (
    <section className="pbstats-section">
      <span className="section-tag">Playbook stats</span>
      <div className="pbstats-grid">
        {stats.map((s) => (
          <div key={s.playbook} className="pbstat-card">
            <h3>{s.playbook.replace(/_/g, " ")}</h3>
            <div className="pbstat-metrics">
              <span>{s.trades} trades</span>
              <span>{fmtPct(s.winRate)} WR</span>
              <span>{fmtNum(s.avgR, 2)}R avg</span>
              <span className={pnlClass(s.netPnl)}>{fmtUsd(s.netPnl)}</span>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
