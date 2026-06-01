#!/usr/bin/env bash
# Push to Encore Cloud (triggers staging/production build).
# Run from Git Bash OUTSIDE Cursor agent:  bash scripts/push-encore.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Git for Windows needs /tmp
if [[ ! -d /tmp ]]; then
  mkdir -p /tmp 2>/dev/null || true
fi

if ! mkdir -p "$ROOT/.tmp"; then
  echo "ERROR: cannot create $ROOT/.tmp"
  echo "Fix: run scripts/fix-encore-permissions.ps1 as Administrator"
  exit 1
fi

export TEMP="$(cd "$ROOT/.tmp" && pwd -W)"
export TMP="$TEMP"
export PATH="$HOME/.encore/bin:$PATH"

echo "TEMP=$TEMP"
echo "Checking Encore auth..."
if ! encore auth whoami; then
  echo "Run: encore auth login"
  exit 1
fi

echo ""
echo "Pushing main -> encore/main ..."
if ! GIT_TRACE=1 git push encore main 2>&1; then
  echo ""
  echo "Push failed."
  echo "If you see encore-token-auth-sentinel-key: TEMP is not writable."
  echo "  Run as Admin: powershell -File scripts/fix-encore-permissions.ps1"
  echo "Or link GitHub in Encore Cloud and: git push origin main"
  exit 1
fi

echo ""
echo "Verifying..."
git fetch encore
LOCAL="$(git rev-parse main)"
REMOTE="$(git rev-parse encore/main)"
echo "main=$LOCAL"
echo "encore/main=$REMOTE"
if [[ "$LOCAL" == "$REMOTE" ]]; then
  echo ""
  echo "OK — Encore remote updated. Check builds:"
  echo "  https://app.encore.cloud/aegis-futures-utk2"
else
  echo ""
  echo "WARN — encore/main still differs from local main"
  git status -sb
  exit 1
fi
