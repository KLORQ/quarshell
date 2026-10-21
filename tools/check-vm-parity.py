#!/usr/bin/env python3
"""Validate the versioned OpenShell VM parity inventory against pinned sources."""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
MANIFEST = ROOT / "quarshell-driver-vm/vm/testdata/openshell-v0.1.4-parity.json"
VALID = {"implemented", "partial", "open", "blocked", "unsupported"}


def anchor_words(reference: str) -> list[str]:
    """Return identifier anchors from a file::symbol reference."""
    _, separator, symbol = reference.partition("::")
    if not separator:
        return []
    return re.findall(r"[A-Za-z_][A-Za-z0-9_]*", symbol)


def has_identifier(source: str, identifier: str) -> bool:
    return re.search(r"\b" + re.escape(identifier) + r"\b", source) is not None


def main() -> int:
    try:
        manifest = json.loads(MANIFEST.read_text())
        version = (ROOT / "tools/upstream/UPSTREAM_VERSION").read_text().strip()
        commit = (ROOT / "tools/upstream/UPSTREAM_SHA").read_text().strip()
    except (OSError, json.JSONDecodeError) as exc:
        print(f"VM parity inventory: {exc}", file=sys.stderr)
        return 1

    baseline = manifest.get("baseline", {})
    problems: list[str] = []
    if manifest.get("schema_version") != 1:
        problems.append("unsupported or missing schema_version")
    if baseline.get("version") != version or baseline.get("commit") != commit:
        problems.append("manifest baseline differs from tools/upstream pin")

    entries = manifest.get("entries")
    if not isinstance(entries, list) or not entries:
        problems.append("entries must be a non-empty array")
        entries = []
    seen: set[str] = set()
    for i, entry in enumerate(entries):
        label = f"entries[{i}]"
        if not isinstance(entry, dict):
            problems.append(f"{label} must be an object")
            continue
        feature = entry.get("id", "")
        if not re.fullmatch(r"vm\.[a-z0-9][a-z0-9.-]+", feature):
            problems.append(f"{label} has invalid feature id {feature!r}")
        if feature in seen:
            problems.append(f"duplicate feature id {feature}")
        seen.add(feature)
        if entry.get("status") not in VALID:
            problems.append(f"{feature}: invalid status {entry.get('status')!r}")
        if not isinstance(entry.get("evidence"), list):
            problems.append(f"{feature}: evidence must be an array")
        upstream = entry.get("upstream", "")
        # Several closely related upstream files may document one feature.
        for ref in upstream.split(";"):
            ref = ref.strip()
            path = ref.split("::", 1)[0]
            source = ROOT / "tools/upstream/openshell-v0.1.4" / path
            if path and not source.exists():
                problems.append(f"{feature}: missing pinned upstream source {path}")
                continue
            if path and source.is_file():
                text = source.read_text(errors="replace")
                missing = [word for word in anchor_words(ref) if not has_identifier(text, word)]
                if missing:
                    problems.append(f"{feature}: upstream anchor(s) {', '.join(missing)} not found in {path}")
        cautem = entry.get("cautem", "")
        for ref in cautem.split(";"):
            ref = ref.strip()
            path = ref.split("::", 1)[0]
            source = ROOT / path
            if path and not source.exists():
                problems.append(f"{feature}: missing Cautem source {path}")
                continue
            if path and source.is_file():
                text = source.read_text(errors="replace")
                missing = [word for word in anchor_words(ref) if not has_identifier(text, word)]
                if missing:
                    problems.append(f"{feature}: Cautem anchor(s) {', '.join(missing)} not found in {path}")
        evidence = entry.get("evidence")
        if isinstance(evidence, list):
            for ref in evidence:
                if not isinstance(ref, str) or not ref.strip():
                    problems.append(f"{feature}: evidence references must be non-empty strings")
                    continue
                path, separator, symbol = ref.partition("::")
                source = ROOT / path.strip()
                if not source.is_file():
                    problems.append(f"{feature}: missing evidence source {path.strip()}")
                    continue
                if separator and not has_identifier(source.read_text(errors="replace"), symbol.strip()):
                    problems.append(f"{feature}: evidence symbol {symbol.strip()!r} not found in {path.strip()}")
        if entry.get("status") == "implemented" and not entry.get("evidence"):
            problems.append(f"{feature}: implemented status requires evidence")

    if problems:
        print("VM parity inventory: FAIL", file=sys.stderr)
        for problem in problems:
            print(f"- {problem}", file=sys.stderr)
        return 1
    print(f"VM parity inventory: PASS ({len(entries)} features, {len(seen)} unique IDs, OpenShell {version} {commit})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
