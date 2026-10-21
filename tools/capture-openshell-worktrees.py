#!/usr/bin/env python3
"""Capture linked worktree state for multi-repository operations."""
from __future__ import annotations
import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OUT = Path(sys.argv[1]).resolve() if len(sys.argv) > 1 else ROOT / "plans/inventory/openshell-current-worktrees.json"

def run(path: Path, *args: str) -> str:
    return subprocess.check_output(["git", "-C", str(path), *args], text=True, stderr=subprocess.DEVNULL).strip()

rows = []
seen = set()
for repo in sorted(ROOT.glob("quarshell-*")):
    if not repo.is_dir() or not (repo / ".git").exists():
        continue
    listing = run(repo, "worktree", "list", "--porcelain")
    for block in listing.split("\n\n"):
        fields = dict(line.split(" ", 1) for line in block.splitlines() if " " in line)
        if "worktree" not in fields:
            continue
        checkout = Path(fields["worktree"])
        key = str(checkout.resolve())
        if key in seen or not checkout.exists():
            continue
        seen.add(key)
        status = run(checkout, "status", "--porcelain=v1", "--untracked-files=all")
        entries = []
        for line in status.splitlines():
            entries.append({"index_status": line[0], "worktree_status": line[1], "path": line[3:]})
        rows.append({
            "repository": repo.name,
            "checkout": str(checkout),
            "is_primary": checkout.resolve() == repo.resolve(),
            "branch": fields.get("branch"),
            "detached": "detached" in fields,
            "head": run(checkout, "rev-parse", "HEAD"),
            "status_entries": len(entries),
            "entries": entries,
        })

payload = {"schema_version": 1, "captured_by": "tools/capture-openshell-worktrees.py",
           "purpose": "preserve linked worktree state at capture time",
           "worktree_count": len(rows), "worktrees": rows}
OUT.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n")
print(f"captured {len(rows)} worktrees into {OUT.relative_to(ROOT)}")
