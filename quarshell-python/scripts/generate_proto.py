#!/usr/bin/env python3
"""Generate Quarshell and pinned OpenShell Python gRPC clients."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


MODULE_ROOT = Path(__file__).resolve().parents[1]
WORKSPACE_ROOT = MODULE_ROOT.parent
UPSTREAM_PROTO = WORKSPACE_ROOT / "quarshell-proto" / "upstream" / "openshell" / "v0.1.4" / "proto"
UPSTREAM_LOCK = UPSTREAM_PROTO / "SOURCE.lock.json"
OPEN_SHELL_OUT = MODULE_ROOT / "quarshell" / "_proto" / "openshell"


def verify_pinned_openshell_source() -> None:
    lock = json.loads(UPSTREAM_LOCK.read_text())
    expected = {entry["path"]: entry["sha256"] for entry in lock["files"]}
    actual = {
        path.relative_to(UPSTREAM_PROTO).as_posix()
        for path in UPSTREAM_PROTO.rglob("*.proto")
    }
    if actual != set(expected):
        missing, extra = sorted(set(expected) - actual), sorted(actual - set(expected))
        raise SystemExit(f"pinned OpenShell proto file set mismatch: missing={missing} extra={extra}")
    for relative, digest in expected.items():
        source = UPSTREAM_PROTO / relative
        actual_digest = hashlib.sha256(source.read_bytes()).hexdigest()
        if actual_digest != digest:
            raise SystemExit(f"pinned OpenShell proto digest mismatch: {relative}")


def generate(proto_root: Path, output: Path, files: list[str]) -> None:
    subprocess.run(
        [
            sys.executable,
            "-m",
            "grpc_tools.protoc",
            f"-I{proto_root}",
            f"--python_out={output}",
            f"--pyi_out={output}",
            f"--grpc_python_out={output}",
            *files,
        ],
        cwd=WORKSPACE_ROOT,
        check=True,
    )


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.parse_args()
    verify_pinned_openshell_source()
    with tempfile.TemporaryDirectory(prefix="quarshell-openshell-proto-") as temporary:
        generated_root = Path(temporary)
        generate(
            UPSTREAM_PROTO,
            generated_root,
            ["openshell.proto", "datamodel.proto", "options.proto", "sandbox.proto"],
        )

        # protoc emits flat imports for the upstream's flat proto layout. Make
        # them package-relative so files work from an installed wheel.
        replacements = ("openshell_pb2", "datamodel_pb2", "options_pb2", "sandbox_pb2")
        generated_files = [
            *generated_root.glob("*_pb2*.py"),
            *generated_root.glob("*_pb2*.pyi"),
        ]
        for path in generated_files:
            contents = path.read_text()
            for module in replacements:
                contents = re.sub(
                    rf"^import {module} as ",
                    f"from . import {module} as ",
                    contents,
                    flags=re.MULTILINE,
                )
            path.write_text(contents)

        OPEN_SHELL_OUT.mkdir(parents=True, exist_ok=True)
        for stale in OPEN_SHELL_OUT.glob("*_pb2*"):
            if stale.is_file():
                stale.unlink()
        for path in generated_files:
            shutil.copy2(path, OPEN_SHELL_OUT / path.name)

    for directory in (
        MODULE_ROOT / "quarshell" / "_proto",
        OPEN_SHELL_OUT,
    ):
        directory.mkdir(parents=True, exist_ok=True)
        (directory / "__init__.py").touch()


if __name__ == "__main__":
    main()
