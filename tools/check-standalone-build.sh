#!/usr/bin/env bash
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STRICT=0
if [[ "${1:-}" == "--strict" ]]; then
	STRICT=1
fi

FAILURES=0
for MANIFEST in "$ROOT"/*/go.mod; do
	DIR="$(dirname "$MANIFEST")"
	NAME="$(basename "$DIR")"
	OUTPUT="$(cd "$DIR" && GOWORK=off GOPROXY=off GOFLAGS=-mod=mod go build ./... 2>&1)"
	if [[ $? -eq 0 ]]; then
		echo "PASS $NAME"
		continue
	fi
	FAILURES=$((FAILURES + 1))
	FIRST_LINE="$(printf '%s\n' "$OUTPUT" | head -n 1)"
	echo "BLOCKED $NAME: $FIRST_LINE"
done
if [[ $FAILURES -gt 0 ]]; then
	echo "standalone build: $FAILURES module(s) blocked (unpublished local dependencies or missing cache)" >&2
	if [[ $STRICT -eq 1 ]]; then
		exit 1
	fi
fi
