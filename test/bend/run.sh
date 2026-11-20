#!/usr/bin/env bash
# Same entry point on macOS/Linux and the GitHub runner; no host Node/Bend.
set -euo pipefail
cd "$(dirname "$0")/../.."
export GOWORK=off GOTOOLCHAIN=auto
exec python3 test/bend/run.py "$@"
