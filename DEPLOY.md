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

### How Encore builds are triggered

Encore Cloud builds when **any one** of these succeeds:

| Method | Trigger | Best for |
|--------|---------|----------|
| **GitHub Actions** (recommended if CLI push fails) | `git push origin main` (backend paths) | Windows / broken local Encore auth |
| **Encore ↔ GitHub link** | `git push origin main` | Zero YAML; configure once in dashboard |
| **Encore git remote** | `git push encore main` | Local dev with working `encore auth login` |

Use **one** auto-deploy method — enabling both GitHub link **and** the Actions workflow causes duplicate builds.

### Auto-deploy via GitHub Actions (recommended)

This repo includes `.github/workflows/encore-deploy.yml`. On every push to `main` (excluding `dashboard/**` and markdown-only changes), GitHub pushes to Encore Cloud for you — no local Encore CLI needed.

**One-time setup:**

1. Open [Encore → Auth Keys](https://app.encore.cloud/aegis-futures-utk2/settings/auth-keys) → **Create key** (reusable).
2. GitHub → [aegis-futures → Settings → Secrets → Actions](https://github.com/prasanthkuna/aegis-futures/settings/secrets/actions) → **New repository secret**
   - Name: `ENCORE_AUTH_KEY`
   - Value: paste the key from step 1
3. Commit and push the workflow (or any backend change) to `main`.
4. Watch the run: [Actions tab](https://github.com/prasanthkuna/aegis-futures/actions) → **Deploy backend to Encore Cloud**
5. Confirm build in [Encore Deployments](https://app.encore.cloud/aegis-futures-utk2).

Manual re-deploy: Actions → **Deploy backend to Encore Cloud** → **Run workflow**.

### Auto-deploy via Encore GitHub link (alternative)

No workflow or auth key — configure in the Encore dashboard:

1. [Encore → Settings → GitHub](https://app.encore.cloud/aegis-futures-utk2) → **Connect Account to GitHub**
2. **Link App to GitHub** → select `prasanthkuna/aegis-futures`
3. **Environments → staging** (or your target env) → **Settings → Branch push** → set `main` → **Save**
4. `git push origin main` triggers Encore builds directly

If you use this method, disable or delete `.github/workflows/encore-deploy.yml` to avoid double deploys.

### Manual deploy (Encore git remote)

**Prerequisite:** Encore CLI must be able to write credentials.

```powershell
encore auth whoami
```

If login fails with `Access is denied` on `.auth_token`, run **as Administrator** (Windows Terminal, not Cursor):

```powershell
cd c:\Users\PrashanthKuna\binance
.\scripts\fix-encore-permissions.ps1
```

Then in a **normal** PowerShell window:

```powershell
encore auth login
cd c:\Users\PrashanthKuna\binance
git push encore main
```

**Git Bash** (if push is silent / no build):

```bash
cd ~/binance
bash scripts/push-encore.sh
```

Or manually:

```bash
mkdir -p .tmp
export TEMP="$(cd .tmp && pwd -W)"
export TMP="$TEMP"
export PATH="$HOME/.encore/bin:$PATH"
git push encore main
```

If you see `encore-token-auth-sentinel-key ... cannot find the file`, `%TEMP%` is not writable — run `scripts/fix-encore-permissions.ps1` as Administrator.

**Verify the push worked** (if this fails, no Encore build ran):

```powershell
git fetch encore
git status -sb
# Should show: ## main...encore/main  (not "ahead 1")
git rev-parse main encore/main
# SHAs must match
```

Then open the app in Encore Cloud → **Deployments** / **Rollouts** and confirm a new build for commit `8207468` (or your latest).

Staging API: `https://staging-aegis-futures-utk2.encr.app`

Health checks:

- `GET /status`
- `GET /dashboard/summary` — account balance
- `GET /radar` — setup radar (regime banner, components, deltas)

## Vercel (dashboard)

Production UI: https://aegis-futures-dashboard.vercel.app

API base URL is set in `dashboard/vercel.json` (staging Encore).

Vercel deploys from GitHub when connected, or manually:

```powershell
cd dashboard
pnpm install
pnpm build
vercel deploy --prod --yes
```

After `git push origin main`, check https://vercel.com/prasanthkunas-projects/aegis-futures-dashboard for a deployment on the same commit SHA as `main`.

## After deploy — test from UI

1. Open the dashboard; confirm balance ~$460 in command center.
2. **Setup radar**: regime banner (CHOP / WATCH / MOMENTUM), gap-to-trade, component bars, gates, score Δ on refresh.
3. Sort **Closest to trade** / filter **Trade + watch**.
4. **Start** only after `AegisTradingEnabled=true` in Encore secrets.
