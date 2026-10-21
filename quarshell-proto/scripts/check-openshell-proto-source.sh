#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
python3 - "$ROOT" <<'PY'
import hashlib, json, sys
from pathlib import Path
root = Path(sys.argv[1]) / "upstream/openshell/v0.1.4/proto"
lock = json.loads((root / "SOURCE.lock.json").read_text())
errors = []
expected = {row["path"]: row["sha256"] for row in lock["files"]}
actual = {p.relative_to(root).as_posix() for p in root.rglob("*.proto")}
if actual != set(expected):
    errors.append(f"file set mismatch: missing={sorted(set(expected)-actual)} extra={sorted(actual-set(expected))}")
for rel, digest in expected.items():
    path = root / rel
    if not path.exists() or hashlib.sha256(path.read_bytes()).hexdigest() != digest:
        errors.append(f"source digest mismatch: {rel}")
if errors:
    raise SystemExit("\n".join(errors))
print(f"verified {len(expected)} pinned OpenShell proto sources")
PY
