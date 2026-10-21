#!/usr/bin/env python3
"""Pin inter-repository GitHub Actions checkouts to tools/modules.lock."""

from __future__ import annotations

import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
LOCK = ROOT / "tools" / "modules.lock"


def pins() -> dict[str, str]:
    result: dict[str, str] = {}
    for raw in LOCK.read_text().splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        module, revision = line.split()
        if not re.fullmatch(r"[0-9a-f]{40}", revision):
            raise SystemExit(f"invalid revision for {module}: {revision}")
        result[f"cautem/{module}"] = revision
    if not result:
        raise SystemExit("tools/modules.lock has no committed source pins; pinning is pending new dependency commits")
    return result


def rewrite(path: pathlib.Path, known: dict[str, str], check: bool) -> int:
    lines = path.read_text().splitlines(keepends=True)
    replacements: dict[int, str] = {}
    insertions: dict[int, str] = {}
    changed = 0
    for index, line in enumerate(lines):
        match = re.match(r"^(\s*)repository:\s*([^\s#]+)", line)
        if not match or match.group(2) not in known:
            continue

        repository = match.group(2)
        revision = known[repository]
        indent = match.group(1)
        newline = "\r\n" if line.endswith("\r\n") else "\n"
        ref_index = None
        for candidate in range(index + 1, len(lines)):
            following = lines[candidate]
            if not following.strip():
                continue
            following_indent = len(following) - len(following.lstrip())
            if following_indent < len(indent):
                break
            if following_indent != len(indent):
                continue
            key = re.match(r"^\s*([A-Za-z_][A-Za-z0-9_-]*):", following)
            if not key:
                continue
            if key.group(1) == "ref":
                ref_index = candidate
                break

        if ref_index is None:
            changed += 1
            if not check:
                insertions[index] = f"{indent}ref: {revision}{newline}"
            continue

        ref_line = lines[ref_index]
        ref = re.match(r"^(\s*)ref:\s*([^\s#]+)\s*(?:#.*)?$", ref_line.rstrip("\r\n"))
        if ref is None or ref.group(2) != revision:
            changed += 1
            if not check:
                ref_newline = "\r\n" if ref_line.endswith("\r\n") else "\n"
                replacements[ref_index] = f"{indent}ref: {revision}{ref_newline}"

    if changed and not check:
        output: list[str] = []
        for index, line in enumerate(lines):
            output.append(replacements.get(index, line))
            if index in insertions:
                output.append(insertions[index])
        path.write_text("".join(output))
    return changed


def main() -> int:
    check = "--check" in sys.argv[1:]
    known = pins()
    changed = 0
    for workflow in sorted(ROOT.glob("quarshell-*/.github/workflows/*.y*ml")):
        changed += rewrite(workflow, known, check)
    if check and changed:
        print(f"{changed} inter-repository checkout(s) do not match tools/modules.lock", file=sys.stderr)
        return 1
    print(f"{'found' if check else 'pinned'} {changed} inter-repository checkout(s)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
