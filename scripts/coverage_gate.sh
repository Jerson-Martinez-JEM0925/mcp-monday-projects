#!/bin/sh
# Coverage gate: fail when a gated package drops below its minimum statement
# coverage. Runs inside the builder image (Go toolchain available), e.g.
#   docker run --rm mcp-monday-projects:checks sh scripts/coverage_gate.sh
# Override the threshold with COVERAGE_MIN (default 80).
set -eu

min="${COVERAGE_MIN:-80}"
packages="./internal/application ./internal/monday"
status=0

for pkg in $packages; do
	line=$(go test -cover "$pkg" 2>&1 | tail -n 1)
	pct=$(printf '%s\n' "$line" | sed -n 's/.*coverage: \([0-9.]*\)% of statements.*/\1/p')
	if [ -z "$pct" ]; then
		echo "FAIL $pkg: could not read coverage from: $line"
		status=1
		continue
	fi
	if awk -v p="$pct" -v m="$min" 'BEGIN { exit !(p + 0 < m + 0) }'; then
		echo "FAIL $pkg: ${pct}% < ${min}%"
		status=1
	else
		echo "ok   $pkg: ${pct}% >= ${min}%"
	fi
done

exit "$status"
