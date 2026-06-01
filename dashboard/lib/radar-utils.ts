import type { RadarItem } from "./types";

const CORE = new Set(["BTCUSDT", "ETHUSDT", "SOLUSDT"]);

const defaultComponents: RadarItem["components"] = {
  volume: 0,
  cvd: 0,
  structure: 0,
  context: 0,
  depth: 0,
  session: 0,
};

const defaultGates: RadarItem["gates"] = {
  minScore: false,
  structure: false,
  flow: false,
  btc: false,
  spread: false,
};

/** Fill fields missing from older/simpler /radar API responses. */
export function normalizeRadarItem(
  raw: Partial<RadarItem> & Pick<RadarItem, "symbol">,
  minScore = 0.78
): RadarItem {
  const tradeScore = raw.tradeScore ?? 0;
  return {
    rank: raw.rank ?? 0,
    symbol: raw.symbol,
    quoteVolume24h: raw.quoteVolume24h ?? 0,
    price: raw.price ?? 0,
    spreadBps: raw.spreadBps ?? 0,
    volumeSurge: raw.volumeSurge ?? 0,
    cvdState: raw.cvdState ?? "",
    takerFlow: raw.takerFlow ?? "",
    oiFundingContext: raw.oiFundingContext ?? "",
    coinGlassScore: raw.coinGlassScore ?? 0,
    sessionScore: raw.sessionScore ?? 0,
    tradeScore,
    decision: raw.decision ?? "skip",
    reason: raw.reason ?? "",
    gapToTrade: raw.gapToTrade ?? Math.max(0, minScore - tradeScore),
    weakestLink: raw.weakestLink ?? "balanced",
    tier: raw.tier ?? raw.decision ?? "—",
    sideHint: raw.sideHint ?? "",
    components: raw.components ?? {
      ...defaultComponents,
      volume: raw.volumeSurge ?? 0,
      session: raw.sessionScore ?? 0,
    },
    gates: raw.gates ?? defaultGates,
    btcRegime: raw.btcRegime ?? "neutral",
    isCore: raw.isCore ?? CORE.has(raw.symbol),
    scoreDelta: raw.scoreDelta ?? 0,
    priceDeltaPct: raw.priceDeltaPct ?? 0,
    surgeDelta: raw.surgeDelta ?? 0,
    willFire: raw.willFire ?? false,
    hasOpenSlot: raw.hasOpenSlot ?? false,
  };
}

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
