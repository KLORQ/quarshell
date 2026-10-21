#!/usr/bin/env python3
"""Capture the pinned CLI's runtime command tree and help flags."""

from __future__ import annotations

import hashlib
import json
import re
import subprocess
import sys
from pathlib import Path


def run(binary: Path, args: list[str]) -> tuple[int, str]:
    proc = subprocess.run([str(binary), *args, "--help"], text=True,
                          stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    return proc.returncode, proc.stdout


def commands(help_text: str) -> list[str]:
    result = []
    in_commands = False
    for line in help_text.splitlines():
        heading = line.strip().rstrip(":")
        if heading.upper() == "COMMANDS" or heading.upper().endswith("COMMANDS"):
            in_commands = True
            continue
        if heading.upper() in {"FLAGS", "EXAMPLES", "ALIAS", "LEARN MORE", "GATEWAY FLAGS", "GLOBAL FLAGS"}:
            in_commands = False
            continue
        if in_commands:
            match = re.match(r"^\s{2}([a-z][a-z0-9-]*)(?::\s+|\s{2,})", line)
            if match:
                result.append(match.group(1))
    return result


def flags(help_text: str) -> list[str]:
    result = []
    for line in help_text.splitlines():
        if re.match(r"^\s+(?:-[A-Za-z],\s*)?--[a-z0-9][a-z0-9-]*\b", line):
            result.append(line.strip().split()[0].rstrip(",."))
            if "," in line.strip().split()[0]:
                result[-1] = line.strip().split()[0].rstrip(",")
                result.append(line.strip().split()[1].rstrip(",."))
    return sorted(set(result))


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("usage: capture-openshell-cli-help.py BINARY OUTPUT.json")
    binary, output = Path(sys.argv[1]).resolve(), Path(sys.argv[2])
    version = subprocess.check_output([str(binary), "--version"], text=True).strip()
    seen: dict[tuple[str, ...], dict] = {}
    rejected = []
    # These root commands are present in source but intentionally omitted from
    # the curated root help display (rule is hidden; workspace is also omitted).
    pending = [tuple(), ("rule",), ("workspace",)]
    while pending:
        path = pending.pop(0)
        if path in seen:
            continue
        code, help_text = run(binary, list(path))
        record = {"command_path": list(path), "exit_code": code,
                  "usage": next((line.strip() for line in help_text.splitlines() if "openshell " in line.strip().lower() and line.strip().lower().startswith(("openshell ", "usage:"))), None),
                  "children": commands(help_text), "flags": flags(help_text),
                  "help_text": help_text,
                  "help_sha256": hashlib.sha256(help_text.encode()).hexdigest()}
        if code != 0:
            rejected.append({"command_path": list(path), "exit_code": code,
                             "message": next((line.strip() for line in help_text.splitlines() if line.strip()), "")})
            continue
        seen[path] = record
        pending.extend(path + (child,) for child in record["children"])
    nodes = list(seen.values())
    result = {"baseline": "OpenShell v0.1.4", "binary": str(binary), "version": version,
              "valid_command_paths": len(nodes), "command_help": nodes,
              "rejected_help_entries": rejected,
              "source": "runtime `openshell --help` recursively invoked; no product operations executed"}
    output.write_text(json.dumps(result, indent=2, ensure_ascii=False) + "\n")
    print(f"captured {len(nodes)} command help pages; {len(rejected)} displayed entries are not executable")


if __name__ == "__main__":
    main()
