#!/usr/bin/env python3
"""Validate the machine-readable evidence matrix."""

from __future__ import annotations

import argparse
import json
import re
import sys
from collections import Counter
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_MATRIX = ROOT / "plans/evidence/matrix.yaml"
DEFAULT_INVENTORY = ROOT / "plans/inventory/openshell-v0.1.4-inventory.json"

RESULTS = {"pass", "fail", "blocked", "unverified"}
LEVELS = {"static", "unit", "integration", "target-host", "release"}
SHA256 = re.compile(r"^[0-9a-f]{64}$")


def fail(errors: list[str], message: str) -> None:
    errors.append(message)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("matrix", nargs="?", type=Path, default=DEFAULT_MATRIX)
    parser.add_argument("--inventory", type=Path, default=DEFAULT_INVENTORY)
    args = parser.parse_args()

    errors: list[str] = []
    if not args.matrix.exists():
        print(f"missing evidence matrix: {args.matrix}", file=sys.stderr)
        return 2
    document = yaml.safe_load(args.matrix.read_text()) or {}
    rows = document.get("rows")
    if not isinstance(rows, list) or not rows:
        print("evidence matrix has no rows", file=sys.stderr)
        return 2

    feature_ids: set[str] = set()
    if args.inventory.exists():
        inventory = json.loads(args.inventory.read_text())
        feature_ids = {item["id"] for item in inventory.get("features", [])}

    ids: set[str] = set()
    counts: Counter[str] = Counter()
    for index, row in enumerate(rows, 1):
        where = f"row {index}"
        if not isinstance(row, dict):
            fail(errors, f"{where}: expected a mapping")
            continue
        row_id = row.get("id")
        if not row_id:
            fail(errors, f"{where}: missing id")
            continue
        where = f"{row_id}"
        if row_id in ids:
            fail(errors, f"{where}: duplicate id")
        ids.add(row_id)
        feature_id = row.get("feature_id")
        if not feature_id:
            fail(errors, f"{where}: missing feature_id")
        elif feature_ids and feature_id not in feature_ids:
            fail(errors, f"{where}: unknown feature_id {feature_id!r}")
        for key in ("owner", "scenario", "required", "backend", "host_profile",
                    "source_digest", "command", "result", "evidence_level", "artifact"):
            if key not in row:
                fail(errors, f"{where}: missing field {key!r}")
        result = row.get("result")
        if result not in RESULTS:
            fail(errors, f"{where}: result must be one of {sorted(RESULTS)}, got {result!r}")
            continue
        counts[result] += 1
        if result == "pass":
            for key in ("backend", "host_profile", "command", "artifact", "evidence_level", "last_verified"):
                if not row.get(key):
                    fail(errors, f"{where}: pass requires {key!r}")
            if row.get("evidence_level") and row["evidence_level"] not in LEVELS:
                fail(errors, f"{where}: unknown evidence_level {row['evidence_level']!r}")
            digest = row.get("source_digest")
            if not digest or not SHA256.match(str(digest)):
                fail(errors, f"{where}: pass requires a 64-hex source_digest")
        artifact = row.get("artifact")
        if artifact and not re.match(r"^[a-z]+://", str(artifact)):
            candidate = Path(str(artifact))
            if not candidate.is_absolute():
                candidate = ROOT / candidate
            if not candidate.exists():
                fail(errors, f"{where}: artifact does not exist: {artifact}")

    if document.get("upstream_sha") and args.inventory.exists():
        expected = json.loads(args.inventory.read_text()).get("baseline", {}).get("commit")
        if expected and document["upstream_sha"] != expected:
            fail(errors, f"upstream_sha {document['upstream_sha']} != inventory baseline {expected}")

    summary = ", ".join(f"{key}={counts[key]}" for key in sorted(RESULTS))
    print(f"evidence rows={len(rows)} {summary}")
    if errors:
        for error in errors:
            print(f"error: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
