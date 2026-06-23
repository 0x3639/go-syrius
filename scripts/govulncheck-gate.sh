#!/usr/bin/env bash
# Fail if govulncheck reports any code-affecting vulnerability whose ID is not
# in .github/govulncheck-allow.txt. govulncheck has no native ignore, so we diff
# the reported IDs against the allowlist.
set -uo pipefail

allow_file="$(dirname "$0")/../.github/govulncheck-allow.txt"
allow="$(grep -oE 'GO-[0-9]+-[0-9]+' "$allow_file" | sort -u)"
found="$(govulncheck ./... 2>/dev/null | grep -oE 'GO-[0-9]+-[0-9]+' | sort -u)"

unexpected="$(comm -23 <(printf '%s\n' "$found") <(printf '%s\n' "$allow"))"

if [ -n "$unexpected" ]; then
  echo "FAIL: govulncheck reported non-allowlisted vulnerabilities:"
  printf '%s\n' "$unexpected"
  echo "If a finding is genuinely accepted, add it to .github/govulncheck-allow.txt WITH justification."
  exit 1
fi

echo "OK: only allowlisted vulnerabilities present:"
printf '%s\n' "$found"
