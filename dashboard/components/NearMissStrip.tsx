"use client";

import type { ProSignal } from "@/lib/types";

type Props = {
  items: ProSignal[];
  floor: number;
};

export function NearMissStrip({ items, floor }: Props) {
  if (!items.length) return null;

  return (
    <section className="nearmiss-section">
      <span className="section-tag">Near miss</span>
      <h2 className="section-title">
        Within 10 of floor <span className="mono">{floor}</span>
      </h2>
      <div className="nearmiss-row">
        {items.map((s) => (
          <div key={s.symbol} className="nearmiss-card">
            <strong>{s.symbol.replace("USDT", "")}</strong>
            <span className="mono str">{s.strength}</span>
            <span className="gap mono">+{s.gapToFloor}</span>
            <span className="pb">{s.playbook.replace(/_/g, " ")}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
