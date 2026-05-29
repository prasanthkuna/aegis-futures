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
encore secret set --dev BinanceAPIKey
encore secret set --dev BinanceAPISecret
encore secret set --dev BinanceUseTestnet
encore secret set --dev CoinglassAPIKey
encore secret set --dev TelegramBotToken
encore secret set --dev TelegramChatID
encore secret set --dev AegisTradingEnabled
encore secret set --dev AegisEnv
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

## Smoke test (once)

1. Set `BinanceUseTestnet=true`, `AegisTradingEnabled=true`
2. Place one micro trade; confirm stop + Telegram
3. Revert to mainnet: `BinanceUseTestnet=false`
