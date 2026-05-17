#!/usr/bin/env bash
# Paper LMS — one-time setup for auto-deploy on push to main.
#
# Generates a dedicated SSH deploy keypair, prints the public key for you
# to install on the demo box, then prompts you for the demo box details
# and writes the five required GitHub Actions secrets via the `gh` CLI.
#
# After this runs successfully, every merge to main triggers the
# `deploy-demo` job in .github/workflows/ci.yml, which SSHes into the
# demo box and invokes scripts/deploy.sh.
#
# Idempotent: re-run any time to rotate the key or update the box address.
#
# Usage:
#   ./scripts/setup-auto-deploy.sh

set -euo pipefail

if ! command -v gh >/dev/null 2>&1; then
  echo "ERROR: gh CLI not installed. Install via 'brew install gh' or see https://cli.github.com/"
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "ERROR: gh CLI not authenticated. Run 'gh auth login' first."
  exit 1
fi

REPO="$(gh repo view --json nameWithOwner -q .nameWithOwner)"
KEYDIR="$(mktemp -d -t paper-deploy-key-XXXXXX)"
trap 'rm -rf "$KEYDIR"' EXIT

KEYFILE="$KEYDIR/paper_lms_deploy"

echo
echo "================================================================="
echo " Paper LMS — Auto-Deploy Setup"
echo "================================================================="
echo " Repo: $REPO"
echo

# ---------------------------------------------------------------------------
# Step 1 — generate dedicated deploy keypair
# ---------------------------------------------------------------------------
echo "[1/4] Generating dedicated ED25519 deploy keypair…"
ssh-keygen -t ed25519 -C "paper-lms-deploy@$(date -u +%Y%m%d)" -f "$KEYFILE" -N "" >/dev/null
echo "    ✓ written to $KEYFILE (this directory is temp and will be deleted)"
echo

# ---------------------------------------------------------------------------
# Step 2 — collect demo box details
# ---------------------------------------------------------------------------
echo "[2/4] Demo box connection details."
echo
read -p "    Demo box hostname or IP (e.g. paper.eduthemes.org or 1.2.3.4): " HOST
read -p "    SSH username on demo box (e.g. deploy, ubuntu, paper): " USERNAME
read -p "    Absolute path to the paper-lms repo on the demo box (e.g. /opt/paper-lms): " REPO_PATH
read -p "    SSH port [22]: " PORT
PORT="${PORT:-22}"
echo

# ---------------------------------------------------------------------------
# Step 3 — instruct user to install public key on demo box
# ---------------------------------------------------------------------------
echo "[3/4] Install the PUBLIC key on the demo box."
echo
echo "    SSH into the demo box (any account that can write to ~$USERNAME/.ssh/),"
echo "    then run:"
echo
echo "    --- BEGIN — copy from here ---"
echo "    mkdir -p ~$USERNAME/.ssh && chmod 700 ~$USERNAME/.ssh"
echo "    cat >> ~$USERNAME/.ssh/authorized_keys <<'PUBKEY'"
cat "$KEYFILE.pub"
echo "PUBKEY"
echo "    chmod 600 ~$USERNAME/.ssh/authorized_keys"
echo "    chown -R $USERNAME:$USERNAME ~$USERNAME/.ssh"
echo "    --- END — copy from here ---"
echo
echo "    Quick sanity check from your laptop (should succeed without prompt):"
echo "      ssh -i $KEYFILE -p $PORT $USERNAME@$HOST 'echo OK && cd $REPO_PATH && git rev-parse --short HEAD'"
echo
read -p "    Press ENTER once the key is installed and the sanity check passes (Ctrl-C to abort): " _

# ---------------------------------------------------------------------------
# Step 4 — write the five repo secrets
# ---------------------------------------------------------------------------
echo
echo "[4/4] Writing repo secrets to $REPO via gh CLI…"
gh secret set DEMO_SSH_HOST   --body "$HOST"
gh secret set DEMO_SSH_USER   --body "$USERNAME"
gh secret set DEMO_SSH_PORT   --body "$PORT"
gh secret set DEMO_REPO_PATH  --body "$REPO_PATH"
gh secret set DEMO_SSH_KEY    --body "$(cat "$KEYFILE")"
echo "    ✓ all five secrets written"
echo
echo "    Secrets stored (private key value not echoed):"
gh secret list | grep -E "^DEMO_" || true
echo

# ---------------------------------------------------------------------------
# Cleanup + final instructions
# ---------------------------------------------------------------------------
echo "================================================================="
echo " ✓ Setup complete."
echo "================================================================="
echo
echo " The temp keyfile is in $KEYDIR and will be deleted automatically"
echo " when this script exits. If you'd like to keep a copy for emergency"
echo " manual access, copy it OUT of the temp directory NOW:"
echo "   cp $KEYFILE ~/.ssh/paper_lms_deploy"
echo "   cp $KEYFILE.pub ~/.ssh/paper_lms_deploy.pub"
echo
echo " To rotate the key in the future, just re-run this script. The old"
echo " line in ~$USERNAME/.ssh/authorized_keys on the demo box becomes"
echo " a no-op (you can clean it up if you care)."
echo
echo " To disable auto-deploy for a specific commit, include [skip deploy]"
echo " in the commit message — the deploy-demo job will be skipped."
echo
