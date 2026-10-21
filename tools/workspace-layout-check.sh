#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work="$root/go.work"
test -f "$work"

for manifest in "$root"/*/go.mod; do
	dir="${manifest%/go.mod}"
	rel="${dir#$root/}"
	entry="$(printf '\t./%s' "$rel")"
	grep -Fq "$entry" "$work" || {
		echo "go.work is missing module $rel" >&2
		exit 1
	}
done

while read -r rel; do
	test -f "$root/${rel#./}/go.mod" || {
		echo "go.work references missing module ${rel#./}" >&2
		exit 1
	}
done < <(awk '
	/^use \(/ { in_use=1; next }
	in_use && /^\)/ { in_use=0; next }
	in_use && /^[[:space:]]+\.\// { print $1 }
' "$work")
