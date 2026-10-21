#!/usr/bin/env python3
"""Validate the module dependency DAG against ADR-0002.

Cycles are always an error. Cross-module imports that violate the ADR are
reported as warnings and become errors with --strict.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

# ADR-0002: module -> forbidden target modules. "*" forbids every sibling.
FORBIDDEN = {
    "quarshell-proto": {"*"},
    "quarshell-core": {"quarshell-proto"},
    "quarshell-sdk": {
        "quarshell-gateway",
        "quarshell-cli",
        "quarshell-runtime",
        "quarshell-driver",
        "quarshell-driver-vm",
        "quarshell-driver-mxc",
        "quarshell-isolation",
        "quarshell-supervisor",
        "quarshell-proxy",
        "quarshell-credentials",
        "quarshell-providers",
    },
}

# ADR-0002: the gateway library must not pull VM FFI or Windows APIs.
GATEWAY_FORBIDDEN = {"quarshell-driver-vm", "quarshell-driver-mxc"}


def inventory() -> dict[str, Path]:
    modules: dict[str, Path] = {}
    for manifest in sorted(ROOT.glob("*/go.mod")):
        text = re.sub(r"//[^\n]*", "", manifest.read_text())
        match = re.search(r"^module\s+(\S+)\s*$", text, re.MULTILINE)
        if not match:
            raise SystemExit(f"invalid module directive: {manifest}")
        modules[match.group(1)] = manifest.parent
    if not modules:
        raise SystemExit("no sibling Go modules found")
    return modules


def module_of(import_path: str, modules: dict[str, Path]) -> str | None:
    matches = [name for name in modules if import_path == name or import_path.startswith(name + "/")]
    if not matches:
        return None
    return max(matches, key=len)


def imports(directory: Path) -> list[tuple[str, list[str]]]:
    completed = subprocess.run(
        ["go", "list", "-f", '{{.ImportPath}}|{{join .Imports " "}}', "./..."],
        cwd=directory,
        capture_output=True,
        text=True,
        check=False,
    )
    if completed.returncode != 0:
        if "matched no packages" in completed.stderr:
            return []
        raise SystemExit(f"{directory.name}: go list failed: {completed.stderr.strip()}")
    result = []
    for line in completed.stdout.splitlines():
        if "|" not in line:
            continue
        package, raw = line.split("|", 1)
        result.append((package, raw.split()))
    return result


def find_cycle(edges: dict[str, set[str]]) -> list[str] | None:
    color = {name: 0 for name in edges}
    stack: list[str] = []

    def visit(name: str) -> list[str] | None:
        color[name] = 1
        stack.append(name)
        for target in sorted(edges[name]):
            if color[target] == 0:
                cycle = visit(target)
                if cycle:
                    return cycle
            elif color[target] == 1:
                start = stack.index(target)
                return stack[start:] + [target]
        stack.pop()
        color[name] = 2
        return None

    for name in sorted(edges):
        if color[name] == 0:
            cycle = visit(name)
            if cycle:
                return cycle
    return None


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--strict", action="store_true", help="treat ADR violations as errors")
    args = parser.parse_args()

    modules = inventory()
    short_of = {full: directory.name for full, directory in modules.items()}
    edges: dict[str, set[str]] = {name: set() for name in short_of.values()}
    violations: list[str] = []

    for name, directory in sorted(modules.items()):
        source = short_of[name]
        for package, imported in imports(directory):
            for import_path in imported:
                target = module_of(import_path, modules)
                if target is None:
                    continue
                target_short = short_of[target]
                if target_short == source:
                    continue
                edges[source].add(target_short)
                forbidden = FORBIDDEN.get(source, set())
                if "*" in forbidden or target_short in forbidden:
                    violations.append(
                        f"ADR-0002: {source} imports {target_short} via {package}"
                    )
                if source == "quarshell-gateway" and target_short in GATEWAY_FORBIDDEN and "/cmd/" not in package:
                    violations.append(
                        f"ADR-0002: gateway library imports {target_short} via {package}"
                    )

    cycle = find_cycle(edges)
    edge_count = sum(len(targets) for targets in edges.values())
    print(f"module DAG: {len(modules)} modules, {edge_count} edges")
    for source in sorted(edges):
        for target in sorted(edges[source]):
            print(f"  {source} -> {target}")

    if cycle:
        print("module DAG: FAIL cycle: " + " -> ".join(cycle), file=sys.stderr)
        return 1

    if violations:
        for violation in violations:
            stream = sys.stderr if args.strict else sys.stdout
            print(f"warning: {violation}", file=stream)
    print(f"module DAG: {'FAIL' if args.strict and violations else 'PASS'} (cycles: 0)")
    return 1 if args.strict and violations else 0


if __name__ == "__main__":
    raise SystemExit(main())
