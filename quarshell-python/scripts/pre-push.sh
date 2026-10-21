#!/usr/bin/env bash
set -euo pipefail
repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"
python_bin=${PYTHON:-python3}
tmp=$(mktemp -d)
had_egg_info=0
had_build=0
[ ! -e quarshell.egg-info ] || had_egg_info=1
[ ! -e build ] || had_build=1
cleanup() {
  rm -rf "$tmp"
  [ "$had_egg_info" -eq 1 ] || rm -rf quarshell.egg-info
  [ "$had_build" -eq 1 ] || rm -rf build
}
trap cleanup EXIT HUP INT TERM
"$python_bin" -m venv "$tmp/venv"
"$tmp/venv/bin/python" -m pip install --disable-pip-version-check --upgrade pip setuptools
"$tmp/venv/bin/python" -m pip install --disable-pip-version-check -e '.[dev]' build
"$tmp/venv/bin/python" scripts/generate_proto.py
git diff --exit-code -- quarshell/_proto
"$tmp/venv/bin/python" -m pytest -q
"$tmp/venv/bin/python" -m build --outdir "$tmp/dist"
printf 'pre-push: PASS\n'
