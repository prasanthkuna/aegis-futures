import type { RadarItem } from "./types";

export type SortMode =
  | "actionability"
  | "gap"
  | "score"
  | "delta"
  | "volume"
  | "surge"
  | "spread";

export type FilterPreset =
  | "all"
  | "actionable"
  | "core"
  | "rotators"
  | "btc_block"
  | "tight_spread";

const decisionPri: Record<string, number> = { trade: 0, watch: 1, skip: 2 };

export function sortRadarItems(items: RadarItem[], mode: SortMode): RadarItem[] {
  const out = [...items];
  out.sort((a, b) => {
    switch (mode) {
      case "gap":
        if (a.gapToTrade !== b.gapToTrade) return a.gapToTrade - b.gapToTrade;
        return b.tradeScore - a.tradeScore;
      case "score":
        return b.tradeScore - a.tradeScore;
      case "delta":
        return b.scoreDelta - a.scoreDelta;
      case "volume":
        return b.quoteVolume24h - a.quoteVolume24h;
      case "surge":
        return b.volumeSurge - a.volumeSurge;
      case "spread":
        return a.spreadBps - b.spreadBps;
      case "actionability":
      default: {
        const pa = decisionPri[a.decision] ?? 9;
        const pb = decisionPri[b.decision] ?? 9;
        if (pa !== pb) return pa - pb;
        if (a.gapToTrade !== b.gapToTrade) return a.gapToTrade - b.gapToTrade;
        return b.tradeScore - a.tradeScore;
      }
    }
  });
  return out.map((row, i) => ({ ...row, rank: i + 1 }));
}

export function filterRadarItems(
  items: RadarItem[],
  preset: FilterPreset
): RadarItem[] {
  switch (preset) {
    case "actionable":
      return items.filter((r) => r.decision === "trade" || r.decision === "watch");
    case "core":
      return items.filter((r) => r.isCore);
    case "rotators":
      return items.filter((r) => !r.isCore);
    case "btc_block":
      return items.filter((r) => r.btcRegime !== "neutral");
    case "tight_spread":
      return items.filter((r) => r.spreadBps <= 5);
    default:
      return items;
  }
}

export function weakestLabel(tag: string): string {
  const map: Record<string, string> = {
    no_surge: "no surge",
    flow_flat: "flow flat",
    no_break: "no break",
    context: "context weak",
    spread: "spread/depth",
    session: "session weak",
    btc_block: "BTC block",
    balanced: "balanced",
  };
  return map[tag] ?? tag;
}

export function gateIcon(pass: boolean): string {
  return pass ? "✓" : "·";
}
