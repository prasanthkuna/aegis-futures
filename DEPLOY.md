# Deploy checklist

## Encore Cloud secrets (required before deploy succeeds)

Open: https://app.encore.cloud/aegis-futures-utk2 → **Settings** → **Secrets**

Add these **5** secrets for **staging** and **production** (same values):

| Secret name | Example value | Notes |
|-------------|---------------|--------|
| `BinanceAPIKey` | your key | Futures API key |
| `BinanceAPISecret` | your secret | Futures API secret |
| `BinanceUseTestnet` | `false` | mainnet |
| `AegisTradingEnabled` | `false` | keep false until dry-run done |
| `AegisEnv` | `production` | |

Do **not** add CoinGlass or Telegram — they were removed from the app definition.

CLI (if Encore API is reachable from your machine):

```powershell
cd c:\Users\PrashanthKuna\binance
echo YOUR_KEY | encore secret set --type prod,dev BinanceAPIKey
echo YOUR_SECRET | encore secret set --type prod,dev BinanceAPISecret
echo false | encore secret set --type prod,dev BinanceUseTestnet
echo false | encore secret set --type prod,dev AegisTradingEnabled
echo production | encore secret set --type prod,dev AegisEnv
git push encore main
```

## After secrets + deploy

Staging API: `https://staging-aegis-futures-utk2.encr.app`

Test: `GET /status`

## Vercel (dashboard only)

```powershell
cd dashboard
pnpm install
```

In Vercel project env:

- `NEXT_PUBLIC_API_BASE_URL` = `https://staging-aegis-futures-utk2.encr.app`

```powershell
vercel --cwd dashboard --prod
```
