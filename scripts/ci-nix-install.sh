#!/usr/bin/env bash
# Official, immutable Nix binary distribution; no third-party caches or tokens.
# Run only on an ephemeral GitHub-hosted x86_64 Linux runner for this comparison.
set -euo pipefail
: "${GITHUB_ACTIONS:?Only the hosted CI installer is supported}"
: "${RUNNER_TEMP:?}"
: "${GITHUB_PATH:?}"
: "${GITHUB_ENV:?}"
: "${RUNNER_ENVIRONMENT:?Only GitHub-hosted runners are supported}"
test "$GITHUB_ACTIONS" = true
test "$RUNNER_ENVIRONMENT" = github-hosted
test "$(uname -s)" = Linux
test "$(uname -m)" = x86_64
if command -v nix >/dev/null; then
  printf 'Unexpected pre-existing Nix installation; refusing to replace it\n' >&2
  exit 1
fi
work=$(mktemp -d "$RUNNER_TEMP/azerlay-nix-install.XXXXXXXX")
trap 'rm -rf -- "$work"' EXIT
curl --fail --location --retry 2 --connect-timeout 20 --max-time 180 \
  https://releases.nixos.org/nix/nix-2.35.2/nix-2.35.2-x86_64-linux.tar.xz \
  --output "$work/nix.tar.xz"
printf '%s  %s\n' 0c3960a9792331a22081c3c7a5d8465db9b17c50b3acdf18587fa4c6f2cb1158 "$work/nix.tar.xz" | sha256sum --check
# The digest is taken from the official 2.35.2 install script and independently
# checked against the downloaded binary archive. It covers its bundled installer.
tar -xJf "$work/nix.tar.xz" -C "$work"
printf 'experimental-features = nix-command flakes\nmax-jobs = auto\n' > "$work/nix.conf"
NIX_INSTALLER_NO_MODIFY_PROFILE=1 sh "$work/nix-2.35.2-x86_64-linux/install" \
  --daemon --yes --no-channel-add --no-modify-profile \
  --daemon-user-count "$(( $(nproc) * 2 ))" --nix-extra-conf-file "$work/nix.conf"
export PATH="/nix/var/nix/profiles/default/bin:$PATH"
test "$(nix --version)" = 'nix (Nix) 2.35.2'
printf '/nix/var/nix/profiles/default/bin\n' >> "$GITHUB_PATH"
printf 'NIX_PROFILES=/nix/var/nix/profiles/default\nNIX_SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt\n' >> "$GITHUB_ENV"
# Check a strict allowlist of settings without printing access-token settings.
nix config show --json | python3 -c '
import json, sys
s = json.load(sys.stdin)
def value(k):
    x = s[k]
    return x.get("value") if isinstance(x, dict) else x
assert value("trusted-users") == ["root"], "Unexpected privileged Nix user"
assert [u.rstrip("/") for u in value("substituters")] == ["https://cache.nixos.org"], "Unexpected substituter"
assert value("require-sigs") is True, "Substitute signatures must remain required"
assert value("sandbox") is True or value("sandbox") == "true", "Sandbox must remain enabled"
print("Nix defaults verified: root-only trust, official signed cache, sandbox enabled")
'
