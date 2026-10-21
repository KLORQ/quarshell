#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
for manifest in "$root"/*/go.mod; do
	printf '%s\n' "${manifest%/go.mod}"
done | sed "s#^$root/##" | sort | paste -sd ' ' -
