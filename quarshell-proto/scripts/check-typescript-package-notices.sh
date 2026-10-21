#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"

cmp LICENSE typescript/LICENSE
cmp NOTICE typescript/NOTICE
printf 'TypeScript npm package license and notice match repository sources\n'
