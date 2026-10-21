#!/usr/bin/env python3
"""Audit Kubernetes/VM backend config keys against the pinned Rust structs.

Missing keys are valid upstream settings the Go backend does not decode yet;
they are reported as warnings unless --strict is set. Extra keys are Go-only
settings and are reported separately.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
UPSTREAM = ROOT / "tools/upstream/openshell-v0.1.4/crates"

BACKENDS = {
    "kubernetes": {
        "rust": UPSTREAM / "openshell-driver-kubernetes/src/config.rs",
        "struct": "KubernetesComputeConfig",
        "go": ROOT / "quarshell-driver/internal/kubernetes/backend_config.go",
    },
    "vm": {
        "rust": UPSTREAM / "openshell-driver-vm/src/driver.rs",
        "struct": "VmDriverConfig",
        "go": ROOT / "quarshell-driver-vm/vm/backend_config.go",
    },
}


def rust_fields(path: Path, name: str) -> set[str]:
    if not path.exists():
        raise SystemExit(f"missing pinned source: {path}")
    text = path.read_text()
    match = re.search(rf"pub struct {name} \{{(.*?)\n\}}", text, re.S)
    if not match:
        raise SystemExit(f"upstream struct not found: {name}")
    fields: set[str] = set()
    for line in match.group(1).splitlines():
        stripped = line.strip()
        if stripped.startswith("pub ") and ":" in stripped:
            field = stripped[4:].split(":", 1)[0].strip()
            if field:
                fields.add(field)
    return fields


def go_keys(path: Path) -> set[str]:
    if not path.exists():
        raise SystemExit(f"missing Go backend config: {path}")
    text = path.read_text()
    keys: set[str] = set()
    for clause in re.findall(r"case\s+((?:\"[^\"]+\"\s*,\s*)*\"[^\"]+\")\s*:", text):
        keys.update(re.findall(r'"([^"]+)"', clause))
    return keys


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--strict", action="store_true", help="fail on missing upstream keys")
    args = parser.parse_args()

    missing_total = 0
    for backend, paths in BACKENDS.items():
        upstream = rust_fields(paths["rust"], paths["struct"])
        go = go_keys(paths["go"])
        missing = sorted(upstream - go)
        extra = sorted(go - upstream)
        missing_total += len(missing)
        for key in missing:
            print(f"warning: {backend}: upstream key {key!r} is not decoded", file=sys.stdout if not args.strict else sys.stderr)
        for key in extra:
            print(f"info: {backend}: Go-only key {key!r}", file=sys.stdout)
        print(f"{backend}: {len(upstream)} upstream keys, {len(go)} Go keys, {len(missing)} missing, {len(extra)} Go-only")
    if missing_total and args.strict:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
