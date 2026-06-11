"use client";

import { fmtTime } from "@/lib/format";
import type { SignalFeedEvent } from "@/lib/types";

type Props = { events: SignalFeedEvent[] };

const kindClass: Record<string, string> = {
  scan: "feed-scan",
  will_fire: "feed-fire",
  execute_ok: "feed-ok",
  execute_fail: "feed-fail",
};

export function SignalFeed({ events }: Props) {
  const rows = [...events].reverse().slice(0, 24);

  return (
    <section className="feed-section">
      <span className="section-tag">Signal feed</span>
      <h2 className="section-title">Engine log</h2>
      <div className="feed-list">
        {rows.map((e, i) => (
          <div key={`${e.at}-${i}`} className={`feed-row ${kindClass[e.kind] ?? ""}`}>
            <span className="feed-time mono">{fmtTime(e.at)}</span>
            <span className="feed-kind">{e.kind.replace(/_/g, " ")}</span>
            {e.symbol && <span className="feed-sym">{e.symbol.replace("USDT", "")}</span>}
            <span className="feed-msg">{e.message}</span>
          </div>
        ))}
        {!rows.length && (
          <div className="feed-empty">Feed populates as the engine scans (every 2s)</div>
        )}
      </div>
    </section>
  );
}
