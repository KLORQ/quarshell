#!/usr/bin/env python3
"""Generate the human-readable evidence status from the machine-readable matrix."""

from __future__ import annotations

import argparse
import sys
from collections import Counter
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_MATRIX = ROOT / "plans/evidence/matrix.yaml"
DEFAULT_OUTPUT = ROOT / "plans/evidence/STATUS.md"


def render(document: dict) -> str:
    rows = document.get("rows") or []
    counts = Counter(row.get("result", "unverified") for row in rows)
    required = [row for row in rows if row.get("required")]
    ready = all(row.get("result") == "pass" for row in required) and bool(required)
    lines = [
        "# Evidence status",
        "",
        "Generated from `matrix.yaml` by `tools/generate-evidence-report.py`.",
        "",
        f"- Baseline: {document.get('baseline', 'unknown')}",
        f"- Upstream SHA: `{document.get('upstream_sha', 'unknown')}`",
        f"- Rows: {len(rows)} (required {len(required)})",
        f"- Promotion: {'READY' if ready else 'NOT READY'}",
        "- Counts: " + ", ".join(f"{key}={counts[key]}" for key in sorted(counts)),
        "",
        "| ID | Owner | Required | Result | Level | Backend | Host | Artifact |",
        "| --- | --- | --- | --- | --- | --- | --- | --- |",
    ]
    for row in rows:
        lines.append(
            "| {id} | {owner} | {required} | {result} | {level} | {backend} | {host} | {artifact} |".format(
                id=row.get("id", ""),
                owner=row.get("owner", ""),
                required="yes" if row.get("required") else "no",
                result=row.get("result", "unverified"),
                level=row.get("evidence_level") or "-",
                backend=row.get("backend") or "-",
                host=row.get("host_profile") or "-",
                artifact=row.get("artifact") or "-",
            )
        )
    lines.append("")
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("matrix", nargs="?", type=Path, default=DEFAULT_MATRIX)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--check", action="store_true", help="fail when the report is stale")
    args = parser.parse_args()

    if not args.matrix.exists():
        print(f"missing evidence matrix: {args.matrix}", file=sys.stderr)
        return 2
    document = yaml.safe_load(args.matrix.read_text()) or {}
    rendered = render(document)
    if args.check:
        if not args.output.exists() or args.output.read_text() != rendered:
            print(f"evidence report is stale: run python3 tools/generate-evidence-report.py", file=sys.stderr)
            return 1
        print(f"evidence report is current: {args.output.relative_to(ROOT)}")
        return 0
    args.output.write_text(rendered)
    print(f"wrote {args.output.relative_to(ROOT)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
