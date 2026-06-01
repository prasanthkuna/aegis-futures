# Aegis Futures V1

Binance USDⓈ-M order-flow momentum bot — Encore Go backend + Next.js dashboard.

## Prerequisites

- Go 1.22+ (`C:\Program Files\Go\bin` on PATH)
- Encore CLI
- Node 22+ / pnpm

## Secrets

```powershell
$env:PATH = "C:\Program Files\Go\bin;$env:PATH"
cd c:\Users\PrashanthKuna\binance
encore secret set --type prod,dev BinanceAPIKey
encore secret set --type prod,dev BinanceAPISecret
echo false | encore secret set --type prod,dev BinanceUseTestnet
echo false | encore secret set --type prod,dev AegisTradingEnabled
echo production | encore secret set --type prod,dev AegisEnv
```

Use `false` for `BinanceUseTestnet` and `AegisTradingEnabled` until ready for live orders.

## Run backend

```powershell
encore run
```

API: http://localhost:4000

## Run dashboard

```powershell
cd dashboard
pnpm install
pnpm dev
```

Open http://localhost:3000

## Deploy backend (Encore Cloud)

Push to `main` on GitHub auto-deploys the backend via [GitHub Actions](.github/workflows/encore-deploy.yml). One-time setup: add an `ENCORE_AUTH_KEY` secret (see [DEPLOY.md](DEPLOY.md)).

## Smoke test (once)

1. Set `BinanceUseTestnet=true`, `AegisTradingEnabled=true`
2. Place one micro trade; confirm stop + Telegram
3. Revert to mainnet: `BinanceUseTestnet=false`
