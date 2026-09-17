#!/usr/bin/env bash
# Privacy guard for the standing pseudonym rule (CLAUDE.md, Phase 26.2): the
# only attribution anywhere in this repo is the GitHub handle, and nothing about
# the maintainer's machine or tailnet belongs in a public codebase.
#
# This exists because both rules were broken in practice and neither was caught
# by review: Phase 34 shipped test fixtures pasted from live `tailscale status`
# output (a local username and a real tailnet MagicDNS domain), and the AUR
# package's first commit carried a real-name-and-employer email in its metadata.
#
# The checks are deliberately STRUCTURAL rather than a denylist of the actual
# values — a guard that names the things it is hiding would leak them itself.

set -uo pipefail

fail=0
note() { printf '::error::%s\n' "$1"; fail=1; }

# Files to scan: everything tracked, minus vendored checksums and this script
# (which necessarily contains the patterns it matches on).
mapfile -t files < <(git ls-files | grep -vE '^(go\.sum|scripts/privacy-check\.sh)$')

# 1. Absolute home directories. Always a leaked local path and a username with it.
if out=$(grep -nIE '/home/[a-z_][a-z0-9_-]*' "${files[@]}" 2>/dev/null); then
  note "Absolute /home/<user> path found — use a relative path or ~."
  echo "$out"
fi

# 2. Real tailnet hostnames. Tailscale MagicDNS names look like
#    <host>.<tailnet-id>.ts.net, where the tailnet id identifies the network.
#    Fictional examples are fine; a real tailnet id is not. The allowlist holds
#    the obviously-fake ones used in fixtures and docs.
if out=$(grep -nIE '[a-z0-9-]+\.[a-z0-9-]+\.ts\.net' "${files[@]}" 2>/dev/null \
         | grep -vE '(example-tailnet|tailnet-demo|\.tailnet\.ts\.net)'); then
  note "Real-looking tailnet MagicDNS name — use example-tailnet or tailnet.ts.net."
  echo "$out"
fi

# 3. Email addresses. Only the GitHub noreply pseudonym address is permitted;
#    anything else is either a real address or an employer domain.
if out=$(grep -nIE '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}' "${files[@]}" 2>/dev/null \
         | grep -vE '(89451493\+Phundahl@users\.noreply\.github\.com|noreply@anthropic\.com|@tailtui\.dev|srv-web-01|db-cluster-prod|field-laptop|demo@|root@|user@|\[user@\]|aur@aur\.archlinux\.org|noreply@github\.com)'); then
  note "Unexpected email address — only the pseudonym noreply address is allowed."
  echo "$out"
fi

# 4. Commit authorship. The AUR leak was metadata, not content, so scanning
#    files alone would have missed it entirely.
# 4. Commit authorship. The AUR leak was metadata, not content, so scanning
#    files alone would have missed it entirely. The AUTHOR must always be the
#    pseudonym; the COMMITTER may additionally be GitHub's own noreply address,
#    which is what a squash-merge stamps and is not contributor-controlled.
if [ -n "${PRIVACY_CHECK_COMMIT_RANGE:-}" ]; then
  ok_author='89451493\+Phundahl@users\.noreply\.github\.com'
  ok_committer="(${ok_author}|noreply@github\.com)"
  if out=$(git log --format='%H author=%ae committer=%ce' "$PRIVACY_CHECK_COMMIT_RANGE" \
           | grep -vE "author=${ok_author} committer=${ok_committer}\$"); then
    note "Commit author is not the pseudonym's noreply address."
    echo "$out"
  fi
fi

if [ "$fail" -eq 0 ]; then
  echo "privacy-check: clean"
fi
exit "$fail"
