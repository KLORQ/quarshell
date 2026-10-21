#!/usr/bin/env bash
set -euo pipefail
repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"
python_bin=${PYTHON:-python3}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
"$python_bin" -m venv "$tmp/venv"
"$tmp/venv/bin/python" -m pip install --disable-pip-version-check -r requirements.txt
"$tmp/venv/bin/mkdocs" build --strict --site-dir "$tmp/site"
printf 'pre-push: PASS\n'
