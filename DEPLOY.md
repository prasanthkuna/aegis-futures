# Production deploy

## Encore Cloud (API + bot)

App: https://app.encore.cloud/aegis-futures-utk2

**Secrets** (staging + production):

| Secret | Value |
|--------|--------|
| `BinanceAPIKey` | Futures API key (unrestricted or whitelisted for Encore egress) |
| `BinanceAPISecret` | API secret |
| `BinanceUseTestnet` | `false` |
| `AegisTradingEnabled` | `false` until ready for live orders |
| `AegisEnv` | `production` |

Deploy:

```powershell
cd c:\Users\PrashanthKuna\binance
git push encore main
```

Staging API: `https://staging-aegis-futures-utk2.encr.app`

Health checks:

- `GET /status`
- `GET /dashboard/summary` — account balance
- `GET /radar` — setup radar (regime banner, components, deltas)

## Vercel (dashboard)

Production UI: https://aegis-futures-dashboard.vercel.app

API base URL is set in `dashboard/next.config.ts` (staging Encore).

```powershell
cd dashboard
pnpm install
pnpm build
vercel deploy --prod --yes
```

## After deploy — test from UI

1. Open the dashboard; confirm balance ~$460 in command center.
2. **Setup radar**: regime banner (CHOP / WATCH / MOMENTUM), gap-to-trade, component bars, gates, score Δ on refresh.
3. Sort **Closest to trade** / filter **Trade + watch**.
4. **Start** only after `AegisTradingEnabled=true` in Encore secrets.
