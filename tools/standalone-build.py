#!/usr/bin/env python3
"""Build every module with GOWORK=off using a local file-based module proxy.

The proxy serves the sibling modules at the versions required by their go.mod
files, so an unpublished workspace can be tested like a published consumer.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
EXCLUDED_DIRS = {".git", ".worktrees", "node_modules", "target", ".idea", ".gode", "vendor", "dist", ".e2e-tmp", "__pycache__"}


def strip_comments(text: str) -> str:
    return re.sub(r"//[^\n]*", "", text)


def discover() -> dict[str, Path]:
    modules: dict[str, Path] = {}
    for manifest in sorted(ROOT.glob("*/go.mod")):
        match = re.search(r"^module\s+(\S+)\s*$", strip_comments(manifest.read_text()), re.MULTILINE)
        if not match:
            raise SystemExit(f"invalid module directive: {manifest}")
        modules[match.group(1)] = manifest.parent
    if not modules:
        raise SystemExit("no sibling modules found")
    return modules


def required_versions(modules: dict[str, Path]) -> dict[str, set[str]]:
    versions: dict[str, set[str]] = defaultdict(set)
    for manifest in ROOT.glob("*/go.mod"):
        text = strip_comments(manifest.read_text())
        for name, version in re.findall(r"^\s*(?:require\s+)?(\S+)\s+(v\S+)\s*$", text, re.MULTILINE):
            if name in modules:
                versions[name].add(version)
    for name in modules:
        versions[name].add("v0.0.0")
    return versions


def escape_module_path(module: str) -> str:
    result = []
    for char in module:
        if char.isupper():
            result.append("!" + char.lower())
        else:
            result.append(char)
    return "".join(result)


def ignore_nested(source_root: Path):
    def ignore(directory: str, names: list[str]) -> set[str]:
        ignored = set()
        current = Path(directory)
        for name in names:
            if name in EXCLUDED_DIRS:
                ignored.add(name)
                continue
            candidate = current / name
            if candidate.is_dir() and current != source_root and (candidate / "go.mod").exists():
                ignored.add(name)
        return ignored

    return ignore


def write_proxy(modules: dict[str, Path], versions: dict[str, set[str]], proxy: Path) -> None:
    for module, version_set in versions.items():
        source = modules[module]
        escaped = escape_module_path(module)
        version_dir = proxy / escaped / "@v"
        version_dir.mkdir(parents=True, exist_ok=True)
        go_mod = (source / "go.mod").read_text()
        (version_dir / "list").write_text("\n".join(sorted(version_set)) + "\n")
        for version in sorted(version_set):
            (version_dir / f"{version}.mod").write_text(go_mod)
            (version_dir / f"{version}.info").write_text(json.dumps({"Version": version, "Time": "2026-01-01T00:00:00Z"}))
            with zipfile.ZipFile(version_dir / f"{version}.zip", "w", zipfile.ZIP_DEFLATED) as archive:
                prefix = f"{module}@{version}/"
                for path in sorted(source.rglob("*")):
                    if path.is_dir() or path.is_symlink():
                        continue
                    relative = path.relative_to(source)
                    if any(part in EXCLUDED_DIRS for part in relative.parts):
                        continue
                    if any((source / Path(*relative.parts[: index + 1]) / "go.mod").exists() for index in range(len(relative.parts) - 1)):
                        continue
                    archive.write(path, prefix + relative.as_posix())


def build(module: str, source: Path, proxy: Path, timeout: float) -> tuple[bool, str]:
    with tempfile.TemporaryDirectory(prefix="quarshell-standalone-") as temporary:
        destination = Path(temporary) / source.name
        shutil.copytree(source, destination, ignore=ignore_nested(source))
        environment = os.environ.copy()
        environment.update(
            {
                "GOWORK": "off",
                "GOPROXY": f"file://{proxy},https://proxy.golang.org",
                "GOSUMDB": "off",
                "GONOSUMDB": "*",
                "GOFLAGS": "-mod=mod",
                "GOTOOLCHAIN": "local",
            }
        )
        completed = subprocess.run(
            ["go", "build", "./..."],
            cwd=destination,
            env=environment,
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
        if completed.returncode == 0:
            return True, ""
        output = (completed.stderr or completed.stdout).strip().splitlines()
        return False, output[0] if output else "go build failed"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--module", help="build only one module directory")
    parser.add_argument("--strict", action="store_true", help="exit 1 when any module fails")
    parser.add_argument("--timeout", type=float, default=180.0)
    parser.add_argument("--keep-proxy", action="store_true", help="keep the generated proxy")
    args = parser.parse_args()

    modules = discover()
    versions = required_versions(modules)
    proxy = Path(tempfile.mkdtemp(prefix="quarshell-module-proxy-"))
    try:
        write_proxy(modules, versions, proxy)
        print(f"local module proxy: {proxy}")
        failures = 0
        for name, source in sorted(modules.items(), key=lambda item: item[1].name):
            if args.module and source.name != args.module:
                continue
            try:
                ok, error = build(name, source, proxy, args.timeout)
            except subprocess.TimeoutExpired:
                ok, error = False, f"timed out after {args.timeout:.0f}s"
            if ok:
                print(f"PASS {source.name}")
            else:
                failures += 1
                print(f"FAIL {source.name}: {error}")
        if failures:
            print(f"standalone build: {failures} module(s) failed", file=sys.stderr)
            return 1 if args.strict else 0
        print("standalone build: all modules passed")
        return 0
    finally:
        if not args.keep_proxy:
            shutil.rmtree(proxy, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())
