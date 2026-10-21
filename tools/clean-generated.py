#!/usr/bin/env python3
"""Remove only known ignored build/cache outputs from product worktrees."""
from __future__ import annotations

import argparse
import os
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parent.parent
GENERATED = {
    ".pytest_cache",
    "__pycache__",
    "build",
    "dist",
    "node_modules",
    "playwright-report",
    ".release-dry-run",
    "site",
    "target",
    "test-results",
}
GENERATED_SUFFIXES = (".egg-info",)
GENERATED_FILES = {"quarshell-gateway"}
GENERATED_FILE_SUFFIXES = (".tsbuildinfo",)
EXCLUDED = {".git", ".venv", "venv"}


def worktrees(repo: Path) -> list[Path]:
    try:
        output = subprocess.check_output(
            ["git", "-C", str(repo), "worktree", "list", "--porcelain"],
            text=True,
            stderr=subprocess.DEVNULL,
        )
    except subprocess.CalledProcessError:
        return []
    paths = []
    for line in output.splitlines():
        if line.startswith("worktree "):
            path = Path(line.removeprefix("worktree ")).resolve()
            if path.is_dir():
                paths.append(path)
    return paths


def ignored(repo: Path, path: Path) -> bool:
    relative = path.relative_to(repo).as_posix()
    return subprocess.run(
        ["git", "-C", str(repo), "check-ignore", "--quiet", "--", relative],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        check=False,
    ).returncode == 0


def size(path: Path) -> int:
    if path.is_file():
        try:
            return path.stat().st_size
        except OSError:
            return 0
    total = 0
    for current, dirs, files in os.walk(path, followlinks=False):
        dirs[:] = [name for name in dirs if not (Path(current) / name).is_symlink()]
        for name in files:
            item = Path(current) / name
            try:
                total += item.lstat().st_size
            except OSError:
                pass
    return total


def discover(roots: list[Path]) -> list[tuple[Path, Path]]:
    found: list[tuple[Path, Path]] = []
    seen: set[Path] = set()
    root_set = set(roots)
    for repo in roots:
        for current, dirs, files in os.walk(repo, followlinks=False):
            current_path = Path(current).resolve()
            dirs[:] = [name for name in dirs if name not in EXCLUDED and not (Path(current) / name).is_symlink()]
            for name in list(dirs):
                candidate = (Path(current) / name).resolve()
                if candidate in root_set and candidate != repo:
                    dirs.remove(name)
            for name in list(dirs):
                candidate = Path(current) / name
                if name not in GENERATED and not name.endswith(GENERATED_SUFFIXES):
                    continue
                if not ignored(repo, candidate):
                    continue
                resolved = candidate.resolve()
                if resolved not in seen:
                    found.append((repo, resolved))
                    seen.add(resolved)
                dirs.remove(name)
            for name in list(files):
                candidate = Path(current) / name
                if candidate.is_symlink():
                    continue
                if name not in GENERATED_FILES and not name.endswith(GENERATED_FILE_SUFFIXES):
                    continue
                if not ignored(repo, candidate):
                    continue
                resolved = candidate.resolve()
                if resolved not in seen:
                    found.append((repo, resolved))
                    seen.add(resolved)
                files.remove(name)
    return found


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true", help="delete the listed ignored build outputs")
    args = parser.parse_args()

    repos = [
        ROOT / name
        for name in (
            "quarshell-cli", "quarshell-core", "quarshell-display", "quarshell-docs",
            "quarshell-driver", "quarshell-gateway", "quarshell-org-profile", "quarshell-proto",
            "quarshell-providers", "quarshell-proxy", "quarshell-python", "quarshell-runtime",
            "quarshell-sdk", "quarshell-slogx", "quarshell-ui",
            "quarshell-python-beta-locked-acceptance",
        )
        if (ROOT / name).is_dir()
    ]
    roots = sorted({path for repo in repos for path in worktrees(repo)}, key=str)
    targets = discover(roots)
    reclaim = sum(size(path) for _, path in targets)
    operation = "REMOVE" if args.apply else "DRY RUN"
    for repo, path in targets:
        print(f"{operation}: {path.relative_to(repo)} ({size(path) / 1024**2:.1f} MiB)")
    print(f"{operation}: {len(targets)} ignored generated paths; {reclaim / 1024**3:.2f} GiB")

    if args.apply:
        for _, path in targets:
            if path.is_dir() and not path.is_symlink():
                shutil.rmtree(path)
            elif path.is_symlink():
                path.unlink()
            elif path.is_file():
                path.unlink()
        print("clean-generated: complete")
    else:
        print("Run with --apply to remove these ignored generated paths.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
