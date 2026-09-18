#!/usr/bin/env bash
# Privacy guard for the standing pseudonym rule (CLAUDE.md): the only
# attribution anywhere in this repo is the GitHub handle, and nothing about a
# maintainer's machine or tailnet belongs in a public codebase.
#
# The checks are deliberately STRUCTURAL rather than a denylist of actual
# values — a guard that named the things it looks for would defeat itself,
# which is also why this file excludes itself from its own scan.
#
# MODES
#   (default) | --tracked     every tracked file (CI)
#   --staged                  staged content only (pre-commit)
#   --commits <tip> <rev>…    a set of commits: content, messages, identity
#
# This runs locally via scripts/githooks/ AND in CI. The local run is the one
# that matters: CI only sees a branch once it is already on the remote.

set -uo pipefail

fail=0
note() { printf '::error::%s\n' "$1"; fail=1; }

mode="${1:---tracked}"; shift || true
commit_range=()

case "$mode" in
  --tracked|"")
    mapfile -t files < <(git ls-files | grep -vE '^(go\.sum|scripts/privacy-check\.sh)$')
    ;;
  --staged)
    mapfile -t files < <(git diff --cached --name-only --diff-filter=ACMR \
                         | grep -vE '^(go\.sum|scripts/privacy-check\.sh)$')
    ;;
  --commits)
    tip="${1:-}"; shift || true
    [ -n "$tip" ] && [ "$#" -gt 0 ] || { echo "usage: $0 --commits <tip> <revspec>..." >&2; exit 2; }
    commit_range=("$@")
    mapfile -t files < <(git log --name-only --pretty=format: "$@" 2>/dev/null \
                         | sort -u | grep -vE '^(go\.sum|scripts/privacy-check\.sh)$')
    # Scan the committed blobs, not the worktree.
    tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
    staged=()
    for f in "${files[@]}"; do
      [ -n "$f" ] || continue
      mkdir -p "$tmp/$(dirname "$f")"
      git show "$tip:$f" > "$tmp/$f" 2>/dev/null && staged+=("$tmp/$f")
    done
    files=("${staged[@]}")
    ;;
  *)
    echo "$0: unknown mode '$mode'" >&2; exit 2
    ;;
esac

# --staged compares the index, so materialise those blobs too.
if [ "$mode" = "--staged" ] && [ "${#files[@]}" -gt 0 ]; then
  tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
  staged=()
  for f in "${files[@]}"; do
    [ -n "$f" ] || continue
    mkdir -p "$tmp/$(dirname "$f")"
    git show ":$f" > "$tmp/$f" 2>/dev/null && staged+=("$tmp/$f")
  done
  files=("${staged[@]}")
fi

# Nothing to inspect (e.g. a docs-only staged change that was filtered out).
if [ "${#files[@]}" -eq 0 ]; then files=(/dev/null); fi

# 1. Absolute home directories. Always a leaked local path and a username with it.
if out=$(grep -nIE '/home/[a-z_][a-z0-9_-]*' "${files[@]}" 2>/dev/null); then
  note "Absolute /home/<user> path found — use a relative path or ~."
  echo "$out"
fi

# 2. Real tailnet hostnames. Tailscale MagicDNS names look like
#    <host>.<tailnet-id>.ts.net, where the tailnet id identifies the network.
#    Fictional examples are fine; a real tailnet id is not. The allowlist holds
#    the obviously-fake ones used in fixtures and docs. Keep it in step with the
#    vocabulary in internal/tailscale/mock.go, which is the source of truth for
#    what a fixture may contain (it uses the fictional *.tailtui.dev domain).
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

# 4. Commit authorship and messages. Metadata is as public as file content and
#    is invisible to any scan of the working tree, so it needs its own check.
#    The AUTHOR must always be the pseudonym; the COMMITTER may additionally be
#    GitHub's own noreply address, which is what a squash-merge stamps and is
#    not contributor-controlled.
#    Range comes from --commits (local hooks) or PRIVACY_CHECK_COMMIT_RANGE (CI).
if [ "${#commit_range[@]}" -gt 0 ] || [ -n "${PRIVACY_CHECK_COMMIT_RANGE:-}" ]; then
  if [ "${#commit_range[@]}" -eq 0 ]; then
    commit_range=("$PRIVACY_CHECK_COMMIT_RANGE")
  fi
  ok_author='89451493\+Phundahl@users\.noreply\.github\.com'
  ok_committer="(${ok_author}|noreply@github\.com)"
  if out=$(git log --format='%H author=%ae committer=%ce' "${commit_range[@]}" \
           | grep -vE "author=${ok_author} committer=${ok_committer}\$"); then
    note "Commit author is not the pseudonym's noreply address."
    echo "$out"
  fi
fi

if [ "$fail" -eq 0 ]; then
  echo "privacy-check: clean"
fi
exit "$fail"
